FROM golang:1.22-alpine AS builder
RUN apk add --no-cache curl
# Install current `pack` and v0.31.0, the last pack version that supports heroku/buildpacks:20 builder
ENV PACK_VER=0.38.0
RUN set -ex && \
    mkdir -p /tmp/legacy-pack /tmp/current-pack && \
    cd /tmp/legacy-pack && \
    curl -sLO "https://github.com/buildpacks/pack/releases/download/v0.31.0/pack-v0.31.0-linux.tgz" && \
    tar xvzf "pack-v0.31.0-linux.tgz" && \
    cd /tmp/current-pack && \
    curl -sLO "https://github.com/buildpacks/pack/releases/download/v$PACK_VER/pack-v$PACK_VER-linux.tgz" && \
    tar xvzf "pack-v$PACK_VER-linux.tgz"

WORKDIR /go/src/github.com/apppackio/codebuild-image/builder
COPY ./builder .
RUN go build -o /go/bin/apppack-builder main.go

FROM docker:26-dind
COPY --from=builder /tmp/legacy-pack/pack /usr/local/bin/pack-legacy
COPY --from=builder /tmp/current-pack/pack /usr/local/bin/pack
RUN apk add --no-cache git
COPY --from=builder /go/bin/apppack-builder /usr/local/bin/apppack-builder
