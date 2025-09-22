# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the AppPack Codebuild Image - a minimal Docker image used by AWS CodeBuild for AppPack application builds. The project contains both the Docker image configuration and a Go-based build orchestration tool.

## Key Commands

### Docker Build
```bash
# Build the Docker image (linux/amd64 platform)
make build

# Build and tag with git version
make build-and-tag

# Push to ECR
make ecr-login  # Login to ECR first
make push       # Push with :builder tag
make push-tag   # Push with git version tag
```

### Go Development (builder/)
```bash
# Run tests for the builder
cd builder && go test ./...

# Run specific test package
cd builder && go test ./build/...

# Build the Go binary
cd builder && go build -o apppack-builder

# Run with verbose/debug output
export APPPACK_DEBUG=1
```

## Architecture

### Core Components

The project consists of two main parts:

1. **Docker Image** - Minimal build environment for AWS CodeBuild
   - Defined in `Dockerfile`
   - Pushed to ECR: `public.ecr.aws/d9q4v8a4/apppack-build`
   - Optimized for fast provisioning (< 25s)

2. **Builder CLI** (`builder/`) - Go application that orchestrates builds
   - Entry point: `builder/main.go` → `cmd/root.go`
   - Commands: `prebuild`, `build`, `postbuild`
   - Uses Cobra for CLI framework

### Build System Architecture

The builder supports two build systems:
- **Docker builds** - Traditional Dockerfile-based builds
- **Buildpack builds** - Using Cloud Native Buildpacks (Heroku/Paketo)

Key configuration files parsed by the builder:
- `apppack.toml` - Primary AppPack configuration
  - Can be read from a custom location via APPPACK_TOML env var
  - If read from custom location, it's automatically copied to `apppack.toml` after build for artifact archival
  - The copy operation is handled by `filesystem.CopyAppPackTomlToDefault()` with proper error handling
- `app.json` - Heroku-compatible app configuration
- `metadata.toml` - Build metadata from buildpacks

### AWS Integration

The builder interacts with AWS services:
- **SSM Parameter Store** - Fetches environment variables from configured paths
- **S3** - Stores build artifacts and cache
- **ECR** - Pushes built container images
- **CloudFormation** - Updates stack parameters

### Package Structure

- `cmd/` - CLI command definitions
- `build/` - Core build logic and configuration parsing
  - `apppacktoml.go`, `appjson.go`, `metadatatoml.go` - Config file parsers
  - `prebuild.go`, `build.go`, `postbuild.go` - Build phase implementations
- `aws/` - AWS service integrations
- `containers/` - Docker and buildpack container operations
- `filesystem/` - Git and file operations
- `shlex/` - Shell command parsing utilities

## Build Process Flow

1. **Prebuild** - Prepares environment, clones repository
2. **Build** - Runs Docker or buildpack build based on configuration
3. **Postbuild** - Pushes image to ECR, runs release tasks, updates CloudFormation

## Default BuildSpec

The following buildspec.yml is used by default in AWS CodeBuild:

```yaml
artifacts:
  files:
  - build.log
  - test.log
  - apppack.toml
  - commit.txt
  name: $CODEBUILD_BUILD_NUMBER
phases:
  build:
    commands: apppack-builder build
  install:
    commands:
    - if [ -z "$CODEBUILD_START_TIME" ]; then exit 0; fi
    - echo "Starting Docker daemon..."
    - nohup /usr/local/bin/dockerd --host=unix:///var/run/docker.sock --host=tcp://127.0.0.1:2375
      --storage-driver=overlay2 --registry-mirror=https://registry.apppackcdn.net
      &> /var/lib/docker.log &
    - for i in $(seq 1 10); do docker info > /dev/null 2>&1 && break || sleep 0.5;
      done
  post_build:
    commands: apppack-builder postbuild
  pre_build:
    commands: apppack-builder prebuild
version: 0.2
```

Key points:
- Docker daemon is started during install phase with AppPack's registry mirror
- The three main phases map to `apppack-builder` subcommands
- Artifacts include build/test logs, apppack.toml, and commit information

## Environment Variables

- `APPPACK_DEBUG` - Enable debug logging
- `APPPACK_TOML` - Path to apppack.toml configuration file
- `ALLOW_EOL_SHIMMED_BUILDER` - Allow end-of-life shimmed builders (for testing)

## Development Guidelines

- **Code Formatting**: Always run `gofumpt` before committing Go code changes
- **Testing**: Verify all tests pass with `go test ./...` after making changes