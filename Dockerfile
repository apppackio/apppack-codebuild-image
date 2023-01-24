FROM docker:20-dind

RUN apk add --no-cache python3 aws-cli curl jq bash git
