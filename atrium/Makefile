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
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -ldflags "-w" -tags "salty insecure azrcr localbuild trcshcurator" -o plugins/deploy/target/trcsh-curator github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
azrtrcshcuratorsha:
	sha256sum plugins/deploy/target/trcsh-curator | cut -d' ' -f1 > plugins/deploy/target/trcsh-curator.sha256
azrtrcshcurator: azrtrcshcuratorbuild azrtrcshcuratorsha

trcshcuratorbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w" -tags "salty insecure dockercr localbuild trcshcurator" -o plugins/deploy/target/trcsh-curator github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
trcshcuratorsha:
	sha256sum plugins/deploy/target/trcsh-curator | cut -d' ' -f1 > plugins/deploy/target/trcsh-curator.sha256
trcshcurator: trcshcuratorbuild trcshcuratorsha

cursorcbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "salty insecure azrcr localbuild trcshcursoraw" -o plugins/deploy/target/trcsh-cursor-aw github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/cursor
cursorcsha:
	sha256sum plugins/deploy/target/trcsh-cursor-aw | cut -d' ' -f1 > plugins/deploy/target/trcsh-cursor-aw.sha256
cursoraw: cursorcbuild cursorcsha

cursorkbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build  -tags "salty insecure azrcr localbuild trcshcursork" -o plugins/deploy/target/trcsh-cursor-k github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/cursor
cursorksha:
	sha256sum plugins/deploy/target/trcsh-cursor-k | cut -d' ' -f1 > plugins/deploy/target/trcsh-cursor-k.sha256
cursork: cursorkbuild cursorksha

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

trcshellqwbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -o plugins/deploy/target/trcshqw -ldflags "-w" -tags "salty memonly" github.com/trimble-oss/tierceron/atrium/vestibulum/hive/trcsh
trcshellqwsha:
	sha256sum plugins/deploy/target/trcshqw | cut -d' ' -f1 > plugins/deploy/target/trcshqw.sha256
trcshellqw: trcshellqwbuild trcshellqwsha

trcshellqkbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=pie -o plugins/deploy/target/trcshqk -ldflags "-w" -tags "salty memonly trcshcursoraw" github.com/trimble-oss/tierceron/atrium/vestibulum/hive/trcsh
trcshellqksha:
	sha256sum plugins/deploy/target/trcshqk | cut -d' ' -f1 > plugins/deploy/target/trcshqk.sha256
trcshellqk: trcshellqkbuild trcshellqksha

trcshellwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -buildmode=pie -ldflags "-w" -tags "salty tc windows azrcr memonly" -o plugins/deploy/target/trcsh.exe github.com/trimble-oss/tierceron/atrium/vestibulum/hive/trcsh

trcshkernelbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o plugins/deploy/target/trcshk -buildmode=pie -ldflags "-w" -tags "salty tc azrcr memonly trcshkernel" github.com/trimble-oss/tierceron/atrium/vestibulum/hive/trcsh
trcshkernelsha:
	sha256sum plugins/deploy/target/trcshk | cut -d' ' -f1 > plugins/deploy/target/trcshk.sha256
trcshkernel: trcshkernelbuild trcshkernelsha

trcshellkernelmbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o plugins/deploy/target/trcshk -buildmode=pie -ldflags "-w" -tags "salty tc azrcr memonly trcshkernel trcshcursoraw" github.com/trimble-oss/tierceron/atrium/vestibulum/hive/trcsh
trcshellkernelmsha:
	sha256sum plugins/deploy/target/trcshk | cut -d' ' -f1 > plugins/deploy/target/trcshk.sha256
trcshellkernelm: trcshellkernelmbuild trcshellkernelmsha

testbuild:
	CGO_ENABLED=0  go build  -o test -ldflags "-w" github.com/trimble-oss/tierceron/atrium/test

localtrcshell:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -ldflags "-w" -tags "salty tc windows dockercr memonly" -o $(GOBIN)/trcsh.exe github.com/trimble-oss/tierceron/atrium/vestibulum/hive/trcsh

fenestra:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/fenestra -tags "insecure fyneboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/fenestra

spiralis:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/spiralis -tags "insecure g3nboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/spiralis

all: certify azrtrcshcurator devplugintrcdb trcshellqw trcshellwin fenestra spiralis

allq: certify cursoraw cursork trcshellqw trcshellqk trcshellkernelm
