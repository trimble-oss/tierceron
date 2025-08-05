GIT ?= git
GO_VARS ?= GOOS=linux GOARCH=amd64 CGO_ENABLED=0
GO ?= go
COMMIT := $(shell $(GIT) rev-parse HEAD)
VERSION ?= $(shell $(GIT) describe --tags ${COMMIT} 2> /dev/null || echo "$(COMMIT)")
BUILD_TIME := $(shell LANG=en_US date +"%F_%T_%z")
ROOT := .
LD_FLAGS := -X $(ROOT).Version=$(VERSION) -X $(ROOT).Commit=$(COMMIT) -X $(ROOT).BuildTime=$(BUILD_TIME)
GOBIN ?= ./bin

.PHONY: help clean 
help:
	@echo "Please use \`make <ROOT>' where <ROOT> is one of"
	@echo "  dependencies to go install the dependencies"
	@echo "  hellocustosworld   to build best example of nute usage"
	@echo "  test         to run unittests"

depend:
	go mod tidy

clean:
	rm -f bin

cleanmutabilissdk:
	rm mutabilissdk/mutabilissdk_grpc.pb.go; rm mutabilissdk/mutabilissdk.pb.go

mutabilissdk: */*.proto
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative mutabilissdk/mutabilissdk.proto

mutabilis:
	go build -buildmode=plugin -trimpath -o="./trcmutabilis.so" -tags "tc azrcr memonly kernel" trcmutabilis.go

