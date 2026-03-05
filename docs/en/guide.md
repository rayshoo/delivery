# Delivery - User Guide

## Overview

Delivery is a gRPC-based tool that automates GitOps manifest repository updates. It consists of a **server** that pre-clones manifest repositories and processes update requests, and a **client** that sends deploy requests from CI pipelines.

### Key Features

- **GitOps controller agnostic** — Works with ArgoCD, Flux, or any Git-based deployment
- **Explicit CI-triggered updates** — No polling; CI pipeline decides exactly what to update and when
- **kustomize + yq support** — Update image tags via kustomize or arbitrary YAML values via yq
- **REST + gRPC** — gRPC for native clients, REST API via grpc-gateway for easy integration
- **Slack notifications** — Send deployment notifications after completion
- **Serial processing** — Worker queue prevents concurrent git conflicts

## Architecture

```
CI Pipeline → Delivery Client → (gRPC/REST) → Delivery Server → Git Repo → GitOps Controller
```

1. CI builds a new image and pushes to registry
2. Delivery client sends an update request to the server
3. Server modifies manifests (kustomize/yq), commits, and pushes
4. GitOps controller detects changes and syncs

## Server

### Configuration

Set via environment variables or `.server.env` file.

| Variable | Default | Description |
|---|---|---|
| `DELIVERY_ADDR` | `0.0.0.0` | Listen address |
| `DELIVERY_PORT` | `12010` | gRPC port |
| `DELIVERY_HTTP_PORT` | `12011` | REST API port |
| `DELIVERY_REPO_LIST_FILE_PATH` | `list.yaml` | Repository list file |
| `DELIVERY_WORK_DIRECTORY` | (required) | Directory for cloned repos |
| `DELIVERY_LOG_LEVEL` | `Info` | Log level |
| `DELIVERY_KUSTOMIZE_PATH` | `/usr/bin/kustomize` | kustomize binary path |
| `DELIVERY_YQ_PATH` | `/usr/bin/yq` | yq binary path |
| `DELIVERY_YAMLFMT_PATH` | `/usr/bin/yamlfmt` | yamlfmt binary path |
| `DELIVERY_DEFAULT_COMMIT_USER_NAME` | `Administrator` | Default git commit user |
| `DELIVERY_DEFAULT_COMMIT_USER_EMAIL` | `admin@example.com` | Default git commit email |
| `DELIVERY_FORCE_CLONE` | `false` | Force re-clone on startup |
| `DELIVERY_ALLOW_EMPTY_COMMIT` | `false` | Allow empty commits |
| `DELIVERY_DEFAULT_PRIVATE_KEY_FILE` | auto-detect | SSH private key path |

### Repository List (`list.yaml`)

```yaml
- name: platform
  url: https://gitlab.example.com/infra/platform
  http:
    username: delivery-bot
    password: ${TOKEN}

- name: apps
  url: git@gitlab.example.com:infra/apps.git
  ssh:
    private-key-file: /root/.ssh/id_ed25519
```

### Running

```bash
# Start server
go run cmd/server/main.go

# Or with make
make start
```

## Client

### Configuration

Set via environment variables or `.client.env` file.

| Variable | Default | Description |
|---|---|---|
| `DELIVERY_SERVER_ADDR` | `127.0.0.1` | Server address |
| `DELIVERY_SERVER_PORT` | `12010` | Server gRPC port |
| `DELIVERY_SERVER_ROOT_CERT` | (none) | TLS root cert path |
| `DELIVERY_LOG_LEVEL` | `Info` | Log level |
| `DELIVERY_COMMIT_PROJECT` | (required) | Source project name |
| `DELIVERY_COMMIT_BRANCH` | (required*) | Source branch |
| `DELIVERY_COMMIT_TAG` | (required*) | Source tag (*branch or tag required) |
| `DELIVERY_COMMIT_SHORT_SHA` | (required) | Source commit SHA |
| `DELIVERY_COMMIT_MESSAGE` | (none) | Commit message |
| `DELIVERY_COMMIT_USER_NAME` | (none) | Commit user name |
| `DELIVERY_COMMIT_USER_EMAIL` | (none) | Commit user email |
| `DELIVERY_SPECS` | (none) | Deploy specs (JSON string) |
| `DELIVERY_SPECS_FILE` | (none) | Deploy specs file (YAML/JSON) |
| `DELIVERY_TIMEOUT` | `10` | Connection timeout (seconds) |
| `DELIVERY_NOTIFY_SPEC` | (none) | Notify spec (JSON string) |
| `DELIVERY_NOTIFY_SPEC_FILE` | (none) | Notify spec file (YAML/JSON) |
| `DELIVERY_NOTIFY_TIMEOUT` | `5` | Notification timeout (seconds) |

### Deploy Specs (`specs.yaml`)

```yaml
- url: https://gitlab.example.com/infra/platform
  updates:
  - branch: main
    paths:
    - path: apps/my-app/dev
      kustomize:
        images:
        - name: registry.example.com/my-app
          newName: registry.example.com/my-app
          newTag: v1.2.3
      yq:
      - file: values.yaml
        key: .image.tag
        value: v1.2.3
```

#### yq Value Types

Values are automatically type-detected:

| Value | Detected Type | yq Expression |
|---|---|---|
| `v1.0.0` | string | `.key = strenv(YQ_VALUE)` |
| `123` | number | `.key = 123` |
| `true` / `false` | boolean | `.key = true` |
| `null` | null | `.key = null` |
| multi-line | string | `.key = strenv(YQ_VALUE)` |

No need to manually wrap string values in quotes.

### REST API

The server provides REST endpoints via grpc-gateway alongside gRPC (default port `12011`).

#### Health Check

```bash
curl http://localhost:12011/api/v1/health?service=delivery
```

Response:
```json
{"status":"SERVING"}
```

#### Deploy

```bash
curl --no-buffer -X POST http://localhost:12011/api/v1/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "commitSpec": {
      "CommitMessage": "update image tag to v1.2.3"
    },
    "deploySpecs": [
      {
        "url": "https://gitlab.example.com/infra/platform",
        "updates": [
          {
            "branch": "main",
            "paths": [
              {
                "path": "apps/my-app/dev",
                "kustomize": {
                  "images": [
                    {
                      "name": "registry.example.com/my-app",
                      "newName": "registry.example.com/my-app",
                      "newTag": "v1.2.3"
                    }
                  ]
                }
              }
            ]
          }
        ]
      }
    ]
  }'
```

The deploy endpoint returns a streaming NDJSON response. Use `--no-buffer` with curl to see real-time progress:

```json
{"result":{"message":"[INFO] try updating the https://gitlab.example.com/infra/platform repository"}}
{"result":{"message":"[INFO] the kustomize command to modify the registry.example.com/my-app image was successful"}}
{"result":{"message":"[INFO] the push to commit updates to branch main in repository https://gitlab.example.com/infra/platform was successful"}}
```

## Container Build

```bash
# Build with docker compose
cd delivery/container
docker compose build

# Build individually
docker build -f delivery/container/server.Dockerfile -t delivery-server .
docker build -f delivery/container/client.Dockerfile -t delivery-client .
```

## Project Structure

```
delivery/
├── api/
│   ├── gen/            # Generated protobuf & gateway code
│   └── proto/          # Proto definitions
├── cmd/
│   ├── server/         # Server entrypoint
│   └── client/         # Client entrypoint
├── delivery/
│   └── container/      # Dockerfiles, compose, build binaries
├── docs/
│   ├── en/             # English documentation
│   └── ko/             # Korean documentation
├── examples/           # Example configuration files
├── internal/
│   ├── server/         # Server internal packages
│   └── client/         # Client internal packages
├── Makefile
└── go.mod
```
