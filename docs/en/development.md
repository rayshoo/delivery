# Delivery - Development Guide

## Prerequisites

### protoc (Protocol Buffers Compiler)

Compiles `.proto` definition files into Go code.

- Download: https://github.com/protocolbuffers/protobuf/releases
- Add to `PATH` after installation

```bash
protoc --version
# libprotoc 34.0
```

### protoc Plugins

```bash
# Go protobuf code generation
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Go gRPC code generation
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# gRPC Gateway (REST → gRPC reverse proxy) code generation
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
```

### google/api Proto Files

Required for gRPC Gateway HTTP annotations. Located in `api/proto/google/api/`.

```bash
mkdir -p api/proto/google/api
curl -sL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto \
  -o api/proto/google/api/annotations.proto
curl -sL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto \
  -o api/proto/google/api/http.proto
```

## Build

```bash
# Build client binary
make build

# Build for all platforms
make build-all

# Build server & client containers
docker build -f delivery/container/server.Dockerfile -t delivery-server .
docker build -f delivery/container/client.Dockerfile -t delivery-client .
```

## Proto Code Generation

```bash
make proto
```

This runs:

```bash
protoc -I api/proto \
  --go_out=api/gen/ --go-grpc_out=api/gen/ --grpc-gateway_out=api/gen/ \
  --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --grpc-gateway_opt=paths=source_relative \
  api/proto/deploy.proto api/proto/health.proto
```

Generated files:
- `api/gen/deploy.pb.go` — protobuf message definitions
- `api/gen/deploy_grpc.pb.go` — gRPC server/client stubs
- `api/gen/deploy.pb.gw.go` — REST gateway reverse proxy
- `api/gen/health.pb.go`, `health_grpc.pb.go`, `health.pb.gw.go` — health check

## Go Module Dependencies

### Direct Dependencies

| Package | Purpose |
|---|---|
| `google.golang.org/grpc` | gRPC framework |
| `google.golang.org/protobuf` | Protocol Buffers runtime |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | REST API gateway |
| `github.com/go-git/go-git/v5` | Git operations (clone, fetch, commit, push) |
| `github.com/fatih/color` | Terminal color output |
| `github.com/joho/godotenv` | .env file loader |
| `github.com/sirupsen/logrus` | Logger |
| `github.com/sirupsen/logrus` | Logging |
| `gopkg.in/yaml.v3` | YAML parsing |

### Server Runtime Tools (in container)

| Tool | Environment Variable | Default Path | Purpose |
|---|---|---|---|
| kustomize | `DELIVERY_KUSTOMIZE_PATH` | `/usr/bin/kustomize` | Update kustomization.yaml image tags |
| yq | `DELIVERY_YQ_PATH` | `/usr/bin/yq` | Update YAML values |
| yamlfmt | `DELIVERY_YAMLFMT_PATH` | `/usr/bin/yamlfmt` | YAML formatting |

## Local Development

```bash
# Start server
make start

# Run client
make client
```
