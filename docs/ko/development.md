# Delivery - 개발 가이드

## 사전 요구사항

### protoc (Protocol Buffers 컴파일러)

`.proto` 정의 파일을 Go 코드로 변환하는 컴파일러입니다.

- 다운로드: https://github.com/protocolbuffers/protobuf/releases
- 설치 후 `PATH`에 추가

```bash
protoc --version
# libprotoc 34.0
```

### protoc 플러그인

```bash
# Go protobuf 코드 생성
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Go gRPC 코드 생성
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# gRPC Gateway (REST → gRPC 변환 프록시) 코드 생성
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
```

### google/api Proto 파일

gRPC Gateway의 HTTP 어노테이션에 필요한 proto 파일입니다. `api/proto/google/api/`에 위치합니다.

```bash
mkdir -p api/proto/google/api
curl -sL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto \
  -o api/proto/google/api/annotations.proto
curl -sL https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto \
  -o api/proto/google/api/http.proto
```

## 빌드

```bash
# 클라이언트 바이너리 빌드
make build

# 전체 플랫폼 빌드
make build-all

# 서버 & 클라이언트 컨테이너 빌드
docker build -f delivery/container/server.Dockerfile -t delivery-server .
docker build -f delivery/container/client.Dockerfile -t delivery-client .
```

## Proto 코드 생성

```bash
make proto
```

위 명령은 아래를 실행합니다:

```bash
protoc -I api/proto \
  --go_out=api/gen/ --go-grpc_out=api/gen/ --grpc-gateway_out=api/gen/ \
  --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --grpc-gateway_opt=paths=source_relative \
  api/proto/deploy.proto api/proto/health.proto
```

생성되는 파일:
- `api/gen/deploy.pb.go` — protobuf 메시지 정의
- `api/gen/deploy_grpc.pb.go` — gRPC 서버/클라이언트 stub
- `api/gen/deploy.pb.gw.go` — REST gateway reverse proxy
- `api/gen/health.pb.go`, `health_grpc.pb.go`, `health.pb.gw.go` — health check 관련

## Go 모듈 의존성

### 직접 의존성

| 패키지 | 용도 |
|---|---|
| `google.golang.org/grpc` | gRPC 프레임워크 |
| `google.golang.org/protobuf` | Protocol Buffers 런타임 |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | REST API gateway |
| `github.com/go-git/go-git/v5` | Git 작업 (clone, fetch, commit, push) |
| `github.com/fatih/color` | 터미널 컬러 출력 |
| `github.com/joho/godotenv` | .env 파일 로드 |
| `github.com/sirupsen/logrus` | 로거 |
| `github.com/sirupsen/logrus` | 로깅 |
| `gopkg.in/yaml.v3` | YAML 파싱 |

### 서버 런타임 도구 (컨테이너 내)

| 도구 | 환경변수 | 기본 경로 | 용도 |
|---|---|---|---|
| kustomize | `DELIVERY_KUSTOMIZE_PATH` | `/usr/bin/kustomize` | kustomization.yaml 이미지 태그 변경 |
| yq | `DELIVERY_YQ_PATH` | `/usr/bin/yq` | YAML 파일 값 변경 |
| yamlfmt | `DELIVERY_YAMLFMT_PATH` | `/usr/bin/yamlfmt` | YAML 포맷팅 |

## 로컬 개발

```bash
# 서버 시작
make start

# 클라이언트 실행
make client
```
