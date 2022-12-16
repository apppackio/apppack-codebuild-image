FROM golang:1.19-alpine as builder
WORKDIR /go/src/github.com/apppackio/codebuild-image/builder
COPY ./builder .
RUN go build -o /go/bin/apppack-builder main.go

FROM docker:20-dind

COPY --from=builder /go/bin/apppack-builder /usr/local/bin/apppack-builder

RUN apk add --no-cache git
