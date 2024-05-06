FROM golang:1.22-alpine as builder
RUN apk add --no-cache curl
# last pack version that supports heroku/buildpacks:20 builder
ENV PACK_VER=0.31.0
RUN set -ex && \
    cd /tmp && \
    curl -sLO "https://github.com/buildpacks/pack/releases/download/v$PACK_VER/pack-v$PACK_VER-linux.tgz" && \
    tar xvzf "pack-v$PACK_VER-linux.tgz"

WORKDIR /go/src/github.com/apppackio/codebuild-image/builder
COPY ./builder .
RUN go build -o /go/bin/apppack-builder main.go

FROM docker:26-dind
COPY --from=builder /tmp/pack /usr/local/bin/pack
RUN apk add --no-cache git
COPY --from=builder /go/bin/apppack-builder /usr/local/bin/apppack-builder
