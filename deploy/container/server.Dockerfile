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
RUN set -eux; \
case "$TARGETARCH" in \
    amd64) K_ARCH=amd64; YQ_ARCH=amd64; YFMT_ARCH=x86_64 ;; \
    arm64) K_ARCH=arm64; YQ_ARCH=arm64; YFMT_ARCH=arm64 ;; \
    *) echo "unsupported arch: $TARGETARCH" >&2; exit 1 ;; \
esac; \
tar -xvf deploy/container/files/kustomize_v5.7.1_linux_${K_ARCH}.tar.gz && \
tar -xvf deploy/container/files/yq_linux_${YQ_ARCH}.tar.gz && \
tar -xvf deploy/container/files/yamlfmt_0.17.2_Linux_${YFMT_ARCH}.tar.gz && \
touch .server.env; set -a; . ./.server.env || true; set +a; \
CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
go build  \
-ldflags "-s -w -X main.version=$VERSION" \
-o build/app cmd/server/main.go

FROM --platform=$BUILDPLATFORM $BASE_IMAGE_NAME:$BASE_IMAGE_TAG
ARG TARGETARCH=amd64
LABEL maintainer="rayshoo <fire@dragonz.dev>"
COPY --from=builder /go/src/build/app /app
COPY --from=builder /go/src/kustomize /usr/bin/kustomize
COPY --from=builder /go/src/yq_linux_$TARGETARCH /usr/bin/yq
COPY --from=builder /go/src/yamlfmt /usr/bin/yamlfmt
RUN chmod +x /usr/bin/kustomize && chmod +x /usr/bin/yq && chmod +x /usr/bin/yamlfmt
ENTRYPOINT ["/app"]