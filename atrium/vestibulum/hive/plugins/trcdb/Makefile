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
	@echo "  depend to go install the dependencies"
	@echo "  trcdbplugin   build trcdb as a plugin"
	@echo "  install       install plugin to plugin run directory"

depend:
	go mod tidy

clean:
	rm -f bin

trcdb:
	$(GO_VARS) $(GO) build -o="$(GOBIN)/trcdb" -ldflags="$(LD_FLAGS)" $(ROOT)/trcdb.go

trcdbplugin:
	go build -buildmode=plugin -trimpath -o="./trcdb.so" -tags "tc memonly trcshkernel" $(ROOT)/trcdb.go
install:
	cp trcdb.so /usr/local/trcshk/plugins

test: *.go */*.go */*/*.go
	$(GO_VARS) $(GO) test -v .
	$(GO_VARS) $(GO) test -v ./tests
