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

$(shell uuidgen > ../buildopts/saltyopts/saltguard.txt)

azrtrcshcuratorbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -ldflags "-w" -tags "salty insecure azrcr localbuild" -o plugins/deploy/target/trcsh-curator github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
azrtrcshcuratorsha:
	sha256sum plugins/deploy/target/trcsh-curator | cut -d' ' -f1 > plugins/deploy/target/trcsh-curator.sha256
azrtrcshcurator: azrtrcshcuratorbuild azrtrcshcuratorsha

trcshcuratorbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w" -tags "salty insecure dockercr localbuild" -o plugins/deploy/target/trcsh-curator github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
trcshcuratorsha:
	sha256sum plugins/deploy/target/trcsh-curator | cut -d' ' -f1 > plugins/deploy/target/trcsh-curator.sha256
trcshcurator: trcshcuratorbuild trcshcuratorsha

cursorcbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "salty insecure azrcr localbuild" -o plugins/deploy/target/trcsh-cursor github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
cursorcsha:
	sha256sum plugins/deploy/target/trcsh-cursor | cut -d' ' -f1 > plugins/deploy/target/trcsh-cursor.sha256
cursorc: cursorcbuild cursorcsha

cursormbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "salty insecure azrcr localbuild trcshm" -o plugins/deploy/target/trcsh-cursor-m github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
cursormsha:
	sha256sum plugins/deploy/target/trcsh-cursor-m | cut -d' ' -f1 > plugins/deploy/target/trcsh-cursor-m.sha256
cursorm: cursormbuild cursormsha

devplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w" -tags "salty insecure localbuild" -o plugins/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/trcdb
devplugintrcdbsha:
	sha256sum plugins/deploy/target/trc-vault-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-plugin.sha256
devplugintrcdb: devplugintrcdbbuild devplugintrcdbsha

harbingplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -ldflags "-w" -tags "salty insecure" -o plugins/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/trcdb
harbingplugintrcdbsha:
	sha256sum plugins/deploy/target/trc-vault-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-plugin.sha256
harbinger: harbingplugintrcdbbuild harbingplugintrcdbsha

certify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(GOBIN)/trcplgtool -tags "memonly azrcr" github.com/trimble-oss/tierceron/atrium/vestibulum/cmd/trcplgtool

localcertify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(GOBIN)/trcplgtool -tags "memonly dockercr" github.com/trimble-oss/tierceron/atrium/vestibulum/cmd/trcplgtool

trcshellbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -o plugins/deploy/target/trcsh -ldflags "-w" -tags "salty memonly" github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh
trcshellsha:
	sha256sum plugins/deploy/target/trcsh | cut -d' ' -f1 > plugins/deploy/target/trcsh.sha256
trcshell: trcshellbuild trcshellsha

trcshellqbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -o plugins/deploy/target/trcshq -ldflags "-w" -tags "salty memonly trcshm" github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh
trcshellqsha:
	sha256sum plugins/deploy/target/trcshq | cut -d' ' -f1 > plugins/deploy/target/trcshq.sha256
trcshellq: trcshellqbuild trcshellqsha

trcshellwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -buildmode=pie -ldflags "-w" -tags "salty tc windows azrcr memonly" -o plugins/deploy/target/trcsh.exe github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh

trcshkernelbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o plugins/deploy/target/trcshk -buildmode=pie -ldflags "-w" -tags "salty tc azrcr memonly kernel" github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh
trcshkernelsha:
	sha256sum plugins/deploy/target/trcshk | cut -d' ' -f1 > plugins/deploy/target/trcshk.sha256
trcshkernel: trcshkernelbuild trcshkernelsha

trcshellkernelmbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o plugins/deploy/target/trcshk -buildmode=pie -ldflags "-w" -tags "salty tc azrcr memonly kernel trcshm" github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh
trcshellkernelmsha:
	sha256sum plugins/deploy/target/trcshk | cut -d' ' -f1 > plugins/deploy/target/trcshk.sha256
trcshellkernelm: trcshellkernelmbuild trcshellkernelmsha

testbuild:
	CGO_ENABLED=0  go build  -o test -ldflags "-w" github.com/trimble-oss/tierceron/atrium/test

localtrcshell:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -ldflags "-w" -tags "salty tc windows dockercr memonly" -o $(GOBIN)/trcsh.exe github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh

fenestra:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/fenestra -tags "insecure fyneboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/fenestra

spiralis:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/spiralis -tags "insecure g3nboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/spiralis

all: certify azrtrcshcurator devplugintrcdb trcshell trcshellwin fenestra spiralis

allq: certify cursorc cursorm trcshellq trcshellkernelm
