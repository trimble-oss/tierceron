GIT ?= git
GO_VARS ?= GOOS=linux GOARCH=amd64 CGO_ENABLED=0
GO ?= go
COMMIT := $(shell $(GIT) rev-parse HEAD)
VERSION ?= $(shell $(GIT) describe --tags ${COMMIT} 2> /dev/null || echo "$(COMMIT)")
BUILD_TIME := $(shell LANG=en_US date +"%F_%T_%z")
ROOT := .
LD_FLAGS := -X $(ROOT).Version=$(VERSION) -X $(ROOT).Commit=$(COMMIT) -X $(ROOT).BuildTime=$(BUILD_TIME)
GOBIN ?= ./bin

.PHONY: help depend clean hiveye install test test-coverage test-race

help:
	@echo "Please use \`make <target>' where <target> is one of"
	@echo "  depend              to go install the dependencies"
	@echo "  hiveye              to build Linux plugin (.so)"
	@echo "  install             to install Linux plugin"
	@echo "  test                to run unittests"
	@echo "  test-coverage       to run unittests with coverage report"
	@echo "  test-race           to run unittests with race detection"

depend:
	GOWORK=off $(GO) mod tidy

clean:
	rm -f $(GOBIN)/hiveye.so coverage.out coverage.html

hiveye:
	@mkdir -p $(GOBIN)
	GOWORK=off GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GO) build -buildmode=plugin -trimpath -o="$(GOBIN)/hiveye.so" -tags "tc azrcr memonly trcshkernel" $(ROOT)/trchiveye.go

install: hiveye
	cp $(GOBIN)/hiveye.so /usr/local/trcshk/plugins

test:
	GOWORK=off $(GO_VARS) $(GO) test -v ./...

test-coverage:
	GOWORK=off $(GO_VARS) $(GO) test -v -cover -coverprofile=coverage.out ./...
	GOWORK=off $(GO) tool cover -html=coverage.out -o coverage.html

test-race:
	GOWORK=off CGO_ENABLED=1 $(GO) test -v -race ./...
