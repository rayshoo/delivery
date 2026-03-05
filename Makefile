ifneq (,$(wildcard ./.env))
	include .env
	export
endif
GIT_VER=$(shell git describe --tags 2>/dev/null)
ifeq (,$(GIT_VER))
GIT_VER=$(VERSION)
endif
.DEFAULT_GOAL := build

COMMAND := delivery
OSS := linux windows darwin
ARCHS := amd64 arm64

imports:
	goimports -l -w .
.PHONY:imports

fmt: imports
	go fmt ./...
.PHONY:fmt

lint: fmt
	golint ./...
.PHONY:lint

vet: fmt
	CGO_ENABLED=0 go vet ./...
.PHONY:vet

build: vet proto
	CGO_ENABLED=0 \
	go build \
	-ldflags "-s -w -X main.command=$${COMMAND} -X main.version=$${GIT_VER}" \
	-o build/$${COMMAND} \
	cmd/client/main.go;
.PHONY:build

build-all: proto
	for OS in ${OSS} ; do \
		for ARCH in ${ARCHS} ; do \
			CGO_ENABLED=0 GOOS=$${OS} GOARCH=$${ARCH} \
			go build \
			-ldflags "-s -w -X main.command=$${COMMAND} -X main.version=$${GIT_VER}" \
			-o build/$${COMMAND}-$${OS}-$${ARCH} \
			cmd/client/main.go; \
		done \
	done
.PHONY:build-all

install: vet proto
	for COMMAND in ${COMMANDS} ; do \
		CGO_ENABLED=0 \
		go build \
		-ldflags "-s -w -X main.command=$${COMMAND} -X main.version=$${GIT_VER}" \
		-o $$GOPATH/bin/$${COMMAND} \
		cmd/$${COMMAND}/main.go; \
	done
.PHONY:install

proto:
	protoc -I api/proto \
		--go_out=api/gen/ --go-grpc_out=api/gen/ --grpc-gateway_out=api/gen/ \
		--go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --grpc-gateway_opt=paths=source_relative \
		api/proto/deploy.proto
.PHONY:proto

start:
	go run cmd/server/main.go
.PHONY:start

client:
	go run cmd/client/main.go
.PHONY:client