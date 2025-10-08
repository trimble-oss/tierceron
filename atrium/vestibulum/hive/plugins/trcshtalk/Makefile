GIT ?= git
GO_VARS ?= GOOS=linux GOARCH=amd64 CGO_ENABLED=0
GO ?= go
COMMIT := $(shell $(GIT) rev-parse HEAD)
VERSION ?= $(shell $(GIT) describe --tags ${COMMIT} 2> /dev/null || echo "$(COMMIT)")
BUILD_TIME := $(shell LANG=en_US date +"%F_%T_%z")
ROOT := .
LD_FLAGS := -X $(ROOT).Version=$(VERSION) -X $(ROOT).Commit=$(COMMIT) -X $(ROOT).BuildTime=$(BUILD_TIME)
GOBIN ?= ./bin

# gRPC / Protobuf
PROTO_FILES := $(wildcard */*.proto)
PROTO_DIR := diagnosticssdk
OUT_DIR := .


.PHONY: help clean 
help:
	@echo "Please use \`make <ROOT>' where <ROOT> is one of"
	@echo "  depend...........install all dependencies"
	@echo "  clean..................remove all binaries"
	@echo "  clean.protobuf.........remove all diagnostic protobuf files"
	@echo "  protobuf...............create all diagnostics protobufs for TrcshTalk"
	@echo "  trcshtalk..............create the TrcshTalk plugin"

depend:
	go mod tidy

clean:
	rm -f bin

clean.protobuf:
	rm $(PROTO_DIR)/*.pb.go

protobuf: $(PROTO_FILES)
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative $(PROTO_FILES)

trcshtalk:
	go build -buildmode=plugin -trimpath -o="./trcshtalk.so" -tags "tc azrcr memonly kernel" trcshtalk.go

