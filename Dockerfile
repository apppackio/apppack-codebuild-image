FROM golang:1.25-alpine AS builder
RUN apk add --no-cache curl
ENV PACK_VER=0.38.0
RUN set -ex && \
    mkdir -p /tmp/pack && \
    cd /tmp/pack && \
    curl -sLO "https://github.com/buildpacks/pack/releases/download/v$PACK_VER/pack-v$PACK_VER-linux.tgz" && \
    tar xvzf "pack-v$PACK_VER-linux.tgz"

WORKDIR /go/src/github.com/apppackio/codebuild-image/builder
COPY ./builder .
RUN go build -o /go/bin/apppack-builder main.go

FROM docker:27-dind
COPY --from=builder /tmp/pack/pack /usr/local/bin/pack
RUN apk add --no-cache git
COPY --from=builder /go/bin/apppack-builder /usr/local/bin/apppack-builder
