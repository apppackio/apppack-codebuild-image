FROM golang:1.19-alpine as builder
RUN apk add --no-cache curl
RUN set -ex && \
    cd /tmp && \
    curl -sLO https://github.com/buildpacks/pack/releases/download/v0.33.2/pack-v0.33.2-linux.tgz && \
    tar xvzf pack-v0.33.2-linux.tgz

WORKDIR /go/src/github.com/apppackio/codebuild-image/builder
COPY ./builder .
RUN go build -o /go/bin/apppack-builder main.go

FROM docker:26-dind
COPY --from=builder /tmp/pack /usr/local/bin/pack
RUN apk add --no-cache git
COPY --from=builder /go/bin/apppack-builder /usr/local/bin/apppack-builder
