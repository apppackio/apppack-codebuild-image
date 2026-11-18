#!/usr/bin/env bash
# Run one integration case: clone a branch of the demo app, build it with the
# candidate builder image, and check the web command postbuild wrote into
# apppack.toml.
#
# The build runs against a docker daemon started inside the builder image
# rather than the runner's own, so the daemon under test matches the one
# CodeBuild provides. The flags mirror the buildspec generated in the backend
# repo (formations/app/build.py, install phase) and must be kept in step with
# it:
#
#   --storage-driver=overlay2  the classic image store, which is what the
#     buildspec pins; a GitHub runner daemon uses containerd's. The two are
#     not interchangeable for either build system: pack exports to the
#     daemon along a different path on the containerd store (where it has
#     also hit engine bugs we never see, e.g. buildpacks/pack#2747), and
#     buildx can only export a local cache there. Without this flag the job
#     exercises a daemon we never ship.
#   --registry-mirror          resolves images through the CDN the way
#     CodeBuild does. The runner's daemon has no mirror configured.
#
# A fresh daemon per run also gives a cold image store, so the buildpack
# prefetch is genuinely exercised instead of finding images an earlier job
# left behind in a shared daemon.
#
# DOCKER_HOST is set explicitly so the CLI in the container talks to the
# socket the daemon above listens on, whatever the base image or the job
# environment might otherwise leave set.
set -euo pipefail

: "${IMAGE:?the builder image under test}"
: "${APPNAME:?the app name to build as}"
: "${EXPECTED_COMMAND:?the services.web.command expected in apppack.toml}"
: "${RUN_ID:?a unique id for this run}"
: "${COMMIT_SHA:?the commit being built}"
: "${DOCKER_REPO:?the ECR repo to push to}"
: "${ARTIFACT_BUCKET:?the S3 bucket for build artifacts}"
BRANCH="${BRANCH:-}"
DEMO_REPO="${DEMO_REPO:-https://github.com/apppackio/apppack-demo-python.git}"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

if [ -n "$BRANCH" ]; then
  git clone --depth 1 --branch "$BRANCH" "$DEMO_REPO" "$workdir/app"
else
  git clone --depth 1 "$DEMO_REPO" "$workdir/app"
fi
cd "$workdir/app"

# The AWS entries are names with no value: docker copies those from this
# process's environment, so the credentials never reach an argument list or
# a log line.
cat > .envfile <<EOF
APPNAME=$APPNAME
CODEBUILD_BUILD_ID=demo-python:$RUN_ID
CODEBUILD_SOURCE_VERSION=$COMMIT_SHA
DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME:-}
DOCKERHUB_ACCESS_TOKEN=${DOCKERHUB_ACCESS_TOKEN:-}
DOCKER_REPO=$DOCKER_REPO
ARTIFACT_BUCKET=$ARTIFACT_BUCKET
AWS_REGION
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
AWS_SESSION_TOKEN
EOF

# The three phases are separated by ; rather than && so a failure still runs
# the rest, the way CodeBuild runs its phases. The container's exit code is
# the last phase's, and postbuild runs the app's own test suite, so dropping
# that code would leave a failing app test invisible to the job. It is held
# until the apppack.toml check below has had its say instead.
phase_status=0
docker run \
  --rm \
  --privileged \
  --env-file .envfile \
  --env DOCKER_HOST=unix:///var/run/docker.sock \
  --volume "$(pwd):/app" \
  --workdir /app \
  "$IMAGE" \
  /bin/sh -c "set -x
    nohup /usr/local/bin/dockerd \
      --host=unix:///var/run/docker.sock \
      --host=tcp://127.0.0.1:2375 \
      --storage-driver=overlay2 \
      --registry-mirror=https://registry.apppackcdn.net \
      > /var/lib/docker.log 2>&1 &
    for i in \$(seq 1 30); do docker info > /dev/null 2>&1 && break || sleep 0.5; done
    docker info > /dev/null 2>&1 || { echo 'dockerd did not start'; cat /var/lib/docker.log; exit 1; }
    docker info --format 'storage-driver={{.Driver}}'
    git config --global --add safe.directory /app
    apppack-builder prebuild
    apppack-builder build
    apppack-builder postbuild" || phase_status=$?

echo "--- apppack.toml ---"
cat apppack.toml

check_status=0
actual="$(python -c 'import tomllib; print(tomllib.load(open("apppack.toml", "rb"))["services"]["web"]["command"])')"
if [ "$actual" != "$EXPECTED_COMMAND" ]; then
  echo "services.web.command does not match" >&2
  printf '  expected: %s\n' "$EXPECTED_COMMAND" >&2
  printf '  actual:   %s\n' "$actual" >&2
  check_status=1
else
  echo "services.web.command matches"
fi

# Both failures are reported before either ends the run, since a phase
# failure usually explains the mismatch that follows it.
if [ "$phase_status" -ne 0 ]; then
  echo "build phases exited $phase_status" >&2
  exit "$phase_status"
fi
exit "$check_status"
