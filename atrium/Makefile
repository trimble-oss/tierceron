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

devplugincarrierbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "insecure azrcr docker localbuild" -o plugins/deploy/target/trc-vault-carrier-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
devplugincarriersha:
	sha256sum plugins/deploy/target/trc-vault-carrier-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-carrier-plugin.sha256
devplugincarrier: devplugincarrierbuild devplugincarriersha

localdevplugincarrierbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "insecure dockercr docker localbuild" -o plugins/deploy/target/trc-vault-carrier-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
localdevplugincarriersha:
	sha256sum plugins/deploy/target/trc-vault-carrier-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-carrier-plugin.sha256
localdevplugincarrier: localdevplugincarrierbuild localdevplugincarriersha

devplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "insecure localbuild" -o plugins/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/trcdb
devplugintrcdbsha:
	sha256sum plugins/deploy/target/trc-vault-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-plugin.sha256
devplugintrcdb: devplugintrcdbbuild devplugintrcdbsha

harbingplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "insecure" -o plugins/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/trcdb
harbingplugintrcdbsha:
	sha256sum plugins/deploy/target/trc-vault-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-plugin.sha256
harbinger: harbingplugintrcdbbuild harbingplugintrcdbsha

certify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(GOBIN)/trcplgtool -tags "memonly azrcr docker" github.com/trimble-oss/tierceron/atrium/vestibulum/cmd/trcplgtool

localcertify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(GOBIN)/trcplgtool -tags "memonly dockercr docker" github.com/trimble-oss/tierceron/atrium/vestibulum/cmd/trcplgtool

trcshellbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o plugins/deploy/target/trcsh -tags "memonly" github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh
trcshellsha:
	sha256sum plugins/deploy/target/trcsh | cut -d' ' -f1 > plugins/deploy/target/trcsh.sha256
trcshell: trcshellbuild trcshellsha

trcshellwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -tags "tc windows azrcr memonly" -o plugins/deploy/target/trcsh.exe github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh

trcshellkernelbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o plugins/deploy/target/trcshk -trimpath -tags "tc azrcr memonly kernel" github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh
trcshellkernelsha:
	sha256sum plugins/deploy/target/trcshk | cut -d' ' -f1 > plugins/deploy/target/trcshk.sha256
trcshellkernel: trcshellkernelbuild trcshellkernelsha

localtrcshell:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -tags "tc windows dockercr memonly" -o $(GOBIN)/trcsh.exe github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh

fenestra:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/fenestra -tags "insecure fyneboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/fenestra

spiralis:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/spiralis -tags "insecure g3nboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/spiralis

all: certify devplugincarrier devplugintrcdb trcshell trcshellwin fenestra spiralis
