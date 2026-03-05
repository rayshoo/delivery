# Delivery - 사용 가이드

## 개요

Delivery는 GitOps 매니페스트 저장소 업데이트를 자동화하는 gRPC 기반 도구입니다. 매니페스트 저장소를 미리 클론해두고 업데이트 요청을 처리하는 **서버**와, CI 파이프라인에서 배포 요청을 보내는 **클라이언트**로 구성됩니다.

### 주요 기능

- **GitOps 컨트롤러 비종속** — ArgoCD, Flux 등 어떤 Git 기반 배포 도구와도 사용 가능
- **CI 주도의 명시적 트리거** — 폴링 없이 CI 파이프라인이 정확한 대상과 타이밍을 결정
- **kustomize + yq 지원** — kustomize로 이미지 태그 변경, yq로 임의의 YAML 값 변경
- **REST + gRPC** — gRPC 네이티브 클라이언트와 grpc-gateway 기반 REST API 동시 제공
- **Slack 알림** — 배포 완료 후 알림 전송
- **직렬 처리** — Worker 큐로 동시 git 충돌 방지

## 아키텍처

```
CI 파이프라인 → Delivery Client → (gRPC/REST) → Delivery Server → Git 저장소 → GitOps 컨트롤러
```

1. CI가 새 이미지를 빌드하고 레지스트리에 푸시
2. Delivery 클라이언트가 서버에 업데이트 요청 전송
3. 서버가 매니페스트 수정(kustomize/yq), 커밋, 푸시
4. GitOps 컨트롤러가 변경을 감지하고 동기화

## 서버

### 설정

환경변수 또는 `.server.env` 파일로 설정합니다.

| 변수 | 기본값 | 설명 |
|---|---|---|
| `DELIVERY_ADDR` | `0.0.0.0` | 리슨 주소 |
| `DELIVERY_PORT` | `12010` | gRPC 포트 |
| `DELIVERY_HTTP_PORT` | `12011` | REST API 포트 |
| `DELIVERY_REPO_LIST_FILE_PATH` | `list.yaml` | 저장소 목록 파일 |
| `DELIVERY_WORK_DIRECTORY` | (필수) | 클론 저장소 경로 |
| `DELIVERY_LOG_LEVEL` | `Info` | 로그 레벨 |
| `DELIVERY_KUSTOMIZE_PATH` | `/usr/bin/kustomize` | kustomize 바이너리 경로 |
| `DELIVERY_YQ_PATH` | `/usr/bin/yq` | yq 바이너리 경로 |
| `DELIVERY_YAMLFMT_PATH` | `/usr/bin/yamlfmt` | yamlfmt 바이너리 경로 |
| `DELIVERY_DEFAULT_COMMIT_USER_NAME` | `Administrator` | 기본 git 커밋 사용자 |
| `DELIVERY_DEFAULT_COMMIT_USER_EMAIL` | `admin@example.com` | 기본 git 커밋 이메일 |
| `DELIVERY_FORCE_CLONE` | `false` | 시작 시 강제 재클론 |
| `DELIVERY_ALLOW_EMPTY_COMMIT` | `false` | 빈 커밋 허용 |
| `DELIVERY_DEFAULT_PRIVATE_KEY_FILE` | 자동 감지 | SSH 개인키 경로 |

### 저장소 목록 (`list.yaml`)

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

### 실행

```bash
# 서버 시작
go run cmd/server/main.go

# 또는 make 사용
make start
```

## 클라이언트

### 설정

환경변수 또는 `.client.env` 파일로 설정합니다.

| 변수 | 기본값 | 설명 |
|---|---|---|
| `DELIVERY_SERVER_ADDR` | `127.0.0.1` | 서버 주소 |
| `DELIVERY_SERVER_PORT` | `12010` | 서버 gRPC 포트 |
| `DELIVERY_SERVER_ROOT_CERT` | (없음) | TLS 루트 인증서 경로 |
| `DELIVERY_LOG_LEVEL` | `Info` | 로그 레벨 |
| `DELIVERY_COMMIT_PROJECT` | (필수) | 소스 프로젝트 이름 |
| `DELIVERY_COMMIT_BRANCH` | (필수*) | 소스 브랜치 |
| `DELIVERY_COMMIT_TAG` | (필수*) | 소스 태그 (*브랜치 또는 태그 중 하나 필수) |
| `DELIVERY_COMMIT_SHORT_SHA` | (필수) | 소스 커밋 SHA |
| `DELIVERY_COMMIT_MESSAGE` | (없음) | 커밋 메시지 |
| `DELIVERY_COMMIT_USER_NAME` | (없음) | 커밋 사용자 이름 |
| `DELIVERY_COMMIT_USER_EMAIL` | (없음) | 커밋 사용자 이메일 |
| `DELIVERY_SPECS` | (없음) | 배포 스펙 (JSON 문자열) |
| `DELIVERY_SPECS_FILE` | (없음) | 배포 스펙 파일 (YAML/JSON) |
| `DELIVERY_TIMEOUT` | `10` | 연결 타임아웃 (초) |
| `DELIVERY_NOTIFY_SPEC` | (없음) | 알림 스펙 (JSON 문자열) |
| `DELIVERY_NOTIFY_SPEC_FILE` | (없음) | 알림 스펙 파일 (YAML/JSON) |
| `DELIVERY_NOTIFY_TIMEOUT` | `5` | 알림 타임아웃 (초) |

### 배포 스펙 (`specs.yaml`)

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

#### yq 값 타입

값의 타입이 자동으로 감지됩니다:

| 값 | 감지 타입 | yq 표현식 |
|---|---|---|
| `v1.0.0` | string | `.key = strenv(YQ_VALUE)` |
| `123` | number | `.key = 123` |
| `true` / `false` | boolean | `.key = true` |
| `null` | null | `.key = null` |
| 멀티라인 | string | `.key = strenv(YQ_VALUE)` |

문자열 값을 수동으로 따옴표로 감쌀 필요 없습니다.

### REST API

서버는 gRPC와 함께 grpc-gateway를 통한 REST 엔드포인트를 제공합니다 (기본 포트 `12011`).

#### 헬스 체크

```bash
curl http://localhost:12011/api/v1/health?service=delivery
```

응답:
```json
{"status":"SERVING"}
```

#### 배포

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

배포 엔드포인트는 스트리밍 NDJSON 응답을 반환합니다. curl에서 `--no-buffer` 옵션을 사용하면 실시간 진행 상황을 확인할 수 있습니다:

```json
{"result":{"message":"[INFO] try updating the https://gitlab.example.com/infra/platform repository"}}
{"result":{"message":"[INFO] the kustomize command to modify the registry.example.com/my-app image was successful"}}
{"result":{"message":"[INFO] the push to commit updates to branch main in repository https://gitlab.example.com/infra/platform was successful"}}
```

## 컨테이너 빌드

```bash
# docker compose 로 빌드
cd delivery/container
docker compose build

# 개별 빌드
docker build -f delivery/container/server.Dockerfile -t delivery-server .
docker build -f delivery/container/client.Dockerfile -t delivery-client .
```

## 프로젝트 구조

```
delivery/
├── api/
│   ├── gen/            # 생성된 protobuf & gateway 코드
│   └── proto/          # Proto 정의 파일
├── cmd/
│   ├── server/         # 서버 엔트리포인트
│   └── client/         # 클라이언트 엔트리포인트
├── delivery/
│   └── container/      # Dockerfile, compose, 빌드용 바이너리
├── docs/
│   ├── en/             # 영문 문서
│   └── ko/             # 한글 문서
├── examples/           # 설정 예시 파일
├── internal/
│   ├── server/         # 서버 내부 패키지
│   └── client/         # 클라이언트 내부 패키지
├── Makefile
└── go.mod
```
