ARG BUILD_BASE_IMAGE_NAME=golang
ARG BUILD_BASE_IMAGE_TAG=1.21.3-alpine3.18
ARG BASE_IMAGE_NAME=alpine
ARG BASE_IMAGE_TAG=latest
ARG BUILDPLATFORM=amd64

FROM --platform=$BUILDPLATFORM $BUILD_BASE_IMAGE_NAME:$BUILD_BASE_IMAGE_TAG AS builder
ARG TARGETARCH=amd64
ARG VERSION=latest
WORKDIR /go/src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN touch .client.env; set -a; source .client.env; set +a; \
CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
go build  \
-ldflags "-s -w -X main.version=$VERSION" \
-o build/app cmd/client/main.go

FROM --platform=$BUILDPLATFORM $BASE_IMAGE_NAME:$BASE_IMAGE_TAG
ARG TARGETARCH=amd64
LABEL maintainer="rayshoo <fire@dragonz.dev>"
COPY --from=builder /go/src/build/app /app
COPY --from=builder /go/src/deploy/container/files/jq-linux-$TARGETARCH /usr/bin/jq
RUN apk add curl
ENTRYPOINT ["/app"]