GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

ifeq ($(GOOS),)  # Check if GOOS is already set
  GOOS:=$(shell echo $(shell uname -s) | tr '[A-Z]' '[a-z]' | tr -d '[:space:]')
endif

$(info GOOS:$(GOOS))

ifeq ($(GOOS),darwin)
  ifeq ($(shell echo $(shell uname -m) | tr '[A-Z]' '[a-z]'), arm64e)  # Check for 32-bit ARM (armv7l)
    GOARCH := arm64
  else
    GOARCH := amd64
  endif
else ifeq ($(GOOS),linux)
  ifeq ($(shell echo $(shell uname -m) | tr '[A-Z]' '[a-z]'), armv7l)  # Check for 32-bit ARM (armv7l)
    GOARCH := arm
  else ifeq ($(shell echo $(shell uname -m) | tr '[A-Z]' '[a-z]'),aarch64)
    GOARCH := arm64
  else
    GOARCH := amd64  # Assuming 64-bit AMD64 by default for Linux
  endif
else
  $(error Unsupported GOOS: $(GOOS))
endif

$(info GOARCH: $(GOARCH))

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go install  -tags "trcname prod" -a -ldflags '-w' github.com/trimble-oss/tierceron/trcweb/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install   github.com/trimble-oss/tierceron/trcweb/apiRouter
fiddler:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install  -tags "azure memonly"  github.com/trimble-oss/tierceron/cmd/trcfiddler
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install -buildmode=pie -tags "azure memonly"  github.com/trimble-oss/tierceron/cmd/trcconfig
configwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -tags "windows azure memonly" -o $(GOBIN)/trcconfig.exe github.com/trimble-oss/tierceron/cmd/trcconfig
configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) go build  -tags "darwin azure memonly" -o $(GOBIN)/trcconfig github.com/trimble-oss/tierceron/cmd/trcconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcinit
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmdp/trcinitp
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) go install  -tags "darwin azure memonly" github.com/trimble-oss/tierceron/cmd/trcinit
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install -buildmode=pie -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) go install -buildmode=pie -tags "darwin azure memonly" github.com/trimble-oss/tierceron/cmd/trcx
xlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build   -buildmode=c-shared -a -ldflags '-w' -tags "azure memonly" -o $(GOBIN)/nc.so github.com/trimble-oss/tierceron/zeroconfiglib
maclib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) go build   -buildmode=c-shared -tags "azure" -o $(GOBIN)/nc.dylib github.com/trimble-oss/tierceron/zeroconfiglib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmdp/trcxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcpub
pubmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) go install -tags "darwin azure memonly" github.com/trimble-oss/tierceron/cmd/trcpub
sub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcsub
ctl:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install -buildmode=pie -tags "memonly tc" github.com/trimble-oss/tierceron/cmd/trcctl
ctlmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=$(GOARCH) go install -buildmode=pie -tags "darwin memonly tc" github.com/trimble-oss/tierceron/cmd/trcctl
ctldebug:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install -buildmode=pie -tags "salty memonly argosystub hardwired trcshkernel" -ldflags='-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=ignore' github.com/trimble-oss/tierceron/cmd/trcctl
descartes:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go install -buildmode=pie -tags "azure memonly"  github.com/trimble-oss/tierceron/cmd/trcdescartes

# Usage:
#   make hivepluginbuild PLUGIN=pluginname
#   make hivepluginrelease PLUGIN=pluginname VERSION=v0.1.0
hivepluginbuild:
	@if [ -z "$(PLUGIN)" ]; then \
		echo "ERROR: PLUGIN must be set, e.g., make hivepluginbuild PLUGIN=pluginname"; \
		exit 1; \
	fi

	# Build the plugin
	@echo "==> Building plugin for $(PLUGIN)..."
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=plugin -tags "azure memonly" -o $(GOBIN)/$(PLUGIN).so github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/$(PLUGIN) || { \
		echo "ERROR: Build failed."; \
		exit 1; \
	}

# Usage:
#   make hivepluginrelease PLUGIN=pluginname VERSION=v0.1.0
hivepluginrelease: hivepluginbuild
	@if [ -z "$(PLUGIN)" ] || [ -z "$(VERSION)" ]; then \
		echo "ERROR: PLUGIN and VERSION must be set, e.g., make hivepluginrelease PLUGIN=pluginname VERSION=v0.1.0"; \
		exit 1; \
	fi

	# If build succeeds, proceed with tagging
	@TAG_PREFIX=$$(echo $(PLUGIN) | tr '/' '-'); \
	echo "==> Tagging $(PLUGIN) with tag $$TAG_PREFIX/$(VERSION)"; \
	cd atrium/vestibulum/hive/plugins/$(PLUGIN) && \
	git tag -a "$$TAG_PREFIX/$(VERSION)" -m "Release $(VERSION) for github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/$(PLUGIN)" && \
	cd - >/dev/null

	@TAG_PREFIX=$$(echo $(PLUGIN) | tr '/' '-'); \
	git push origin "$$TAG_PREFIX/$(VERSION)"

gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

cleancache:
	go clean -cache
	go clean -modcache

all: api config seed x xlib pub sub
