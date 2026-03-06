# Delivery - Quick Start Guide

This guide walks through setting up and running the Delivery server and client step by step.

## Overview

```
1. Configure Server → 2. Run Server → 3. Send deploy requests from Client (CI)
```

---

## 1. Server Setup

The server pre-clones manifest repositories and processes client requests to modify YAML, commit, and push.

### 1-1. Create Repository List

Define the Git repositories the server will manage in `list.yaml`.

**HTTPS authentication:**
```yaml
- name: platform
  url: https://github.com/my-org/k8s-manifests.git
  http:
    username: deploy-bot
    password: ghp_xxxxxxxxxxxxxxxxxxxx
```

**SSH authentication:**
```yaml
- name: platform
  url: git@github.com:my-org/k8s-manifests.git
  ssh:
    private-key-file: /root/.ssh/id_ed25519
```

> Use GitHub PAT, GitLab Access Token, etc. for the `password` field.

### 1-2. Server Configuration

Create a `.server.env` file. Minimum required settings:

```bash
# Directory to store cloned repositories (required)
DELIVERY_WORK_DIRECTORY=/var/lib/delivery/repos

# Repository list file path
DELIVERY_REPO_LIST_FILE_PATH=list.yaml
```

Optional settings:
```bash
# Ports (defaults: gRPC 12010, REST 12011)
DELIVERY_PORT=12010
DELIVERY_HTTP_PORT=12011

# Log level (Trace, Debug, Info, Warn, Error)
DELIVERY_LOG_LEVEL=Info

# Force re-clone on startup
DELIVERY_FORCE_CLONE=false

# Default commit user
DELIVERY_DEFAULT_COMMIT_USER_NAME=delivery-bot
DELIVERY_DEFAULT_COMMIT_USER_EMAIL=delivery@example.com
```

### 1-3. Run the Server

**Run directly:**
```bash
go run cmd/server/main.go
```

**Run with Docker:**
```bash
docker run -d \
  -v /path/to/list.yaml:/list.yaml \
  -v /path/to/.server.env:/.server.env \
  -v /var/lib/delivery/repos:/var/lib/delivery/repos \
  -v /root/.ssh/id_ed25519:/root/.ssh/id_ed25519:ro \
  -p 12010:12010 \
  -p 12011:12011 \
  rayshoo/delivery:server-latest
```

### 1-4. Verify

```bash
# Health check via REST API
curl http://localhost:12011/api/v1/health?service=delivery

# Expected response
{"status":"SERVING"}
```

---

## 2. Client Setup (CI Pipeline)

The client runs in CI pipelines and sends deploy requests to the server.

### 2-1. Create Deploy Specs

Define what to update in `specs.yaml`.

**Update image tags with kustomize:**
```yaml
- url: https://github.com/my-org/k8s-manifests.git
  updates:
  - branch: main
    paths:
    - path: apps/my-app/overlays/dev
      kustomize:
        images:
        - name: my-registry.io/my-app
          newName: my-registry.io/my-app
          newTag: v1.2.3
```

**Update arbitrary YAML values with yq:**
```yaml
- url: https://github.com/my-org/k8s-manifests.git
  updates:
  - branch: main
    paths:
    - path: apps/my-app/overlays/dev
      yq:
      - file: values.yaml
        key: .image.tag
        value: v1.2.3
      - file: values.yaml
        key: .replicaCount
        value: "3"
```

> kustomize and yq can be used together within the same path.

### 2-2. Option A — Client Binary (gRPC)

Set environment variables and run the client.

```bash
export DELIVERY_SERVER_ADDR=delivery.example.com
export DELIVERY_SERVER_PORT=12010
export DELIVERY_COMMIT_PROJECT=my-app
export DELIVERY_COMMIT_BRANCH=main
export DELIVERY_COMMIT_SHORT_SHA=abc1234
export DELIVERY_COMMIT_MESSAGE="deploy: update my-app to v1.2.3"
export DELIVERY_SPECS_FILE=specs.yaml

# Run client
delivery
```

**GitLab CI example:**
```yaml
deploy:
  stage: deploy
  image: rayshoo/delivery:client-latest
  variables:
    DELIVERY_SERVER_ADDR: delivery.example.com
    DELIVERY_SERVER_PORT: "12010"
    DELIVERY_COMMIT_PROJECT: $CI_PROJECT_NAME
    DELIVERY_COMMIT_BRANCH: $CI_COMMIT_BRANCH
    DELIVERY_COMMIT_SHORT_SHA: $CI_COMMIT_SHORT_SHA
    DELIVERY_COMMIT_MESSAGE: "deploy: $CI_PROJECT_NAME $CI_COMMIT_SHORT_SHA"
    DELIVERY_COMMIT_USER_NAME: $GITLAB_USER_NAME
    DELIVERY_COMMIT_USER_EMAIL: $GITLAB_USER_EMAIL
    DELIVERY_SPECS_FILE: specs.yaml
  script:
    - /app
```

**GitHub Actions example:**
```yaml
- name: Deploy
  uses: docker://rayshoo/delivery:client-latest
  env:
    DELIVERY_SERVER_ADDR: delivery.example.com
    DELIVERY_SERVER_PORT: "12010"
    DELIVERY_COMMIT_PROJECT: ${{ github.event.repository.name }}
    DELIVERY_COMMIT_BRANCH: ${{ github.ref_name }}
    DELIVERY_COMMIT_SHORT_SHA: ${{ github.sha }}
    DELIVERY_COMMIT_MESSAGE: "deploy: ${{ github.event.repository.name }} ${{ github.sha }}"
    DELIVERY_SPECS_FILE: specs.yaml
```

### 2-3. Option B — REST API (curl)

Deploy without a gRPC client using just curl.

```bash
curl --no-buffer -X POST http://delivery.example.com:12011/api/v1/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "commitSpec": {
      "CommitMessage": "deploy: update my-app to v1.2.3"
    },
    "deploySpecs": [
      {
        "url": "https://github.com/my-org/k8s-manifests.git",
        "updates": [
          {
            "branch": "main",
            "paths": [
              {
                "path": "apps/my-app/overlays/dev",
                "kustomize": {
                  "images": [
                    {
                      "name": "my-registry.io/my-app",
                      "newName": "my-registry.io/my-app",
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

Streaming response:
```json
{"result":{"message":"[INFO] try updating the https://github.com/my-org/k8s-manifests.git repository"}}
{"result":{"message":"[INFO] the kustomize command to modify the my-registry.io/my-app image was successful"}}
{"result":{"message":"[INFO] the push to commit updates to branch main was successful"}}
```

---

## 3. Slack Notifications (Optional)

To send Slack notifications after deployment, configure a notify spec.

**notify.yaml:**
```yaml
notify:
  slack:
  - url: https://slack.com/api/chat.postMessage
    token: ${SLACK_TOKEN}
    data: |
      channel: #deployments
      text: "deployed my-app v1.2.3"
```

```bash
export DELIVERY_NOTIFY_SPEC_FILE=notify.yaml
export DELIVERY_NOTIFY_TIMEOUT=5
```

---

## Summary

| Component | Required Files | Role |
|---|---|---|
| Server | `list.yaml`, `.server.env` | Manage repos, modify/commit/push manifests |
| Client | `specs.yaml`, `.client.env` (or env vars) | Send deploy requests from CI |
| REST API | None | Direct requests via curl (no client needed) |
