# Delivery - 빠른 시작 가이드

이 가이드는 Delivery 서버와 클라이언트를 처음부터 설정하고 실행하는 과정을 단계별로 안내합니다.

## 전체 흐름

```
1. 서버 설정 → 2. 서버 실행 → 3. 클라이언트(CI)에서 배포 요청
```

---

## 1. 서버 설정

서버는 매니페스트 저장소를 미리 클론해두고, 클라이언트의 요청을 받아 YAML 수정 → 커밋 → 푸시를 수행합니다.

### 1-1. 저장소 목록 파일 작성

서버가 관리할 Git 저장소를 `list.yaml`에 정의합니다.

**HTTPS 인증:**
```yaml
- name: platform
  url: https://github.com/my-org/k8s-manifests.git
  http:
    username: deploy-bot
    password: ghp_xxxxxxxxxxxxxxxxxxxx
```

**SSH 인증:**
```yaml
- name: platform
  url: git@github.com:my-org/k8s-manifests.git
  ssh:
    private-key-file: /root/.ssh/id_ed25519
```

> `password` 필드에는 GitHub PAT, GitLab Access Token 등을 사용합니다.

### 1-2. 서버 환경 설정

`.server.env` 파일을 생성합니다. 최소 필수 설정:

```bash
# 클론한 저장소를 저장할 디렉터리 (필수)
DELIVERY_WORK_DIRECTORY=/var/lib/delivery/repos

# 저장소 목록 파일 경로
DELIVERY_REPO_LIST_FILE_PATH=list.yaml
```

선택 설정:
```bash
# 포트 (기본값: gRPC 12010, REST 12011)
DELIVERY_PORT=12010
DELIVERY_HTTP_PORT=12011

# 로그 레벨 (Trace, Debug, Info, Warn, Error)
DELIVERY_LOG_LEVEL=Info

# 시작 시 저장소 강제 재클론
DELIVERY_FORCE_CLONE=false

# 커밋 사용자 기본값
DELIVERY_DEFAULT_COMMIT_USER_NAME=delivery-bot
DELIVERY_DEFAULT_COMMIT_USER_EMAIL=delivery@example.com
```

### 1-3. 서버 실행

**바이너리로 실행:**
```bash
go run cmd/server/main.go
```

**컨테이너로 실행:**
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

### 1-4. 서버 동작 확인

```bash
# REST API로 헬스 체크
curl http://localhost:12011/api/v1/health?service=delivery

# 정상 응답
{"status":"SERVING"}
```

---

## 2. 클라이언트 설정 (CI 파이프라인)

클라이언트는 CI 파이프라인에서 실행되며, 서버에 배포 요청을 보냅니다.

### 2-1. 배포 스펙 파일 작성

`specs.yaml`에 어떤 저장소의 어떤 파일을 어떻게 수정할지 정의합니다.

**kustomize로 이미지 태그 변경:**
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

**yq로 임의의 YAML 값 변경:**
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

> kustomize와 yq는 같은 path 내에서 함께 사용할 수 있습니다.

### 2-2. 방법 A — 클라이언트 바이너리 사용 (gRPC)

CI 환경변수를 설정하고 클라이언트를 실행합니다.

```bash
export DELIVERY_SERVER_ADDR=delivery.example.com
export DELIVERY_SERVER_PORT=12010
export DELIVERY_COMMIT_PROJECT=my-app
export DELIVERY_COMMIT_BRANCH=main
export DELIVERY_COMMIT_SHORT_SHA=abc1234
export DELIVERY_COMMIT_MESSAGE="deploy: update my-app to v1.2.3"
export DELIVERY_SPECS_FILE=specs.yaml

# 클라이언트 실행
delivery
```

**GitLab CI 예시:**
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

**GitHub Actions 예시:**
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

### 2-3. 방법 B — REST API 사용 (curl)

gRPC 클라이언트 없이 curl만으로도 배포할 수 있습니다.

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

실시간 스트리밍 응답:
```json
{"result":{"message":"[INFO] try updating the https://github.com/my-org/k8s-manifests.git repository"}}
{"result":{"message":"[INFO] the kustomize command to modify the my-registry.io/my-app image was successful"}}
{"result":{"message":"[INFO] the push to commit updates to branch main was successful"}}
```

---

## 3. Slack 알림 (선택)

배포 완료 후 Slack 알림을 보내려면 알림 스펙을 설정합니다.

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

## 요약

| 구성 요소 | 필요한 파일 | 역할 |
|---|---|---|
| 서버 | `list.yaml`, `.server.env` | 저장소 관리, 매니페스트 수정/커밋/푸시 |
| 클라이언트 | `specs.yaml`, `.client.env` (또는 환경변수) | CI에서 배포 요청 전송 |
| REST API | 없음 | curl로 직접 요청 (클라이언트 불필요) |
