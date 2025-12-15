GIT ?= git
GO_VARS ?= GOOS=linux GOARCH=amd64 CGO_ENABLED=0
GO ?= go
COMMIT := $(shell $(GIT) rev-parse HEAD)
VERSION ?= $(shell $(GIT) describe --tags ${COMMIT} 2> /dev/null || echo "$(COMMIT)")
BUILD_TIME := $(shell LANG=en_US date +"%F_%T_%z")
ROOT := .
LD_FLAGS := -X $(ROOT).Version=$(VERSION) -X $(ROOT).Commit=$(COMMIT) -X $(ROOT).BuildTime=$(BUILD_TIME)
GOBIN ?= ./bin

.PHONY: help clean test test-coverage test-race procurator install
help:
	@echo "Please use \`make <target>' where <target> is one of"
	@echo "  depend              to go install the dependencies"
	@echo "  procurator          to build Linux plugin (.so)"
	@echo "  install             to install Linux plugin"
	@echo "  test                to run unittests"
	@echo "  test-coverage       to run unittests with coverage report"
	@echo "  test-race           to run unittests with race detection"
	@echo ""
	@echo "Note: For Windows, compile procurator directly into trcsh using the 'hardwired' architecture"
	@echo "      (Go plugins are not supported on Windows)"

depend:
	go mod tidy

clean:
	rm -f procurator.so

procurator:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildmode=plugin -o="./procurator.so" -tags "tc azrcr memonly trcshkernel" $(ROOT)/procurator.go

install:
	cp procurator.so /usr/local/trcshk/plugins

test:
	$(GO_VARS) $(GO) test -v ./...

test-coverage:
	$(GO_VARS) $(GO) test -v -cover -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

test-race:
	CGO_ENABLED=1 $(GO) test -v -race ./...
