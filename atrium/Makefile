GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

devplugincarrierbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -tags "insecure azrcr" -o plugins/deploy/target/trc-vault-carrier-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/carrier
devplugincarriersha:
	sha256sum plugins/deploy/target/trc-vault-carrier-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-carrier-plugin.sha256
devplugincarrier: devplugincarrierbuild devplugincarriersha

devplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build  -tags "insecure" -o plugins/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/trcdb
devplugintrcdbsha:
	sha256sum plugins/deploy/target/trc-vault-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-plugin.sha256
devplugintrcdb: devplugintrcdbbuild devplugintrcdbsha

harbingplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build  -tags "insecure" -o plugins/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/atrium/vestibulum/plugins/trcdb
harbingplugintrcdbsha:
	sha256sum plugins/deploy/target/trc-vault-plugin | cut -d' ' -f1 > plugins/deploy/target/trc-vault-plugin.sha256
harbinger: harbingplugintrcdbbuild harbingplugintrcdbsha

certify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/trcplgtool -tags "memonly azrcr" github.com/trimble-oss/tierceron/atrium/vestibulum/cmd/trcplgtool
trcshell: 
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/trcsh -tags "memonly" github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh
trcshellwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -tags "tc windows azrcr memonly" -o $(GOBIN)/trcsh.exe github.com/trimble-oss/tierceron/atrium/vestibulum/shell/trcsh

fenestra:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/fenestra -tags "insecure fyneboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/fenestra

spiralis:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/spiralis -tags "insecure g3nboot argosystub tc" -ldflags="$(LD_FLAGS)" github.com/trimble-oss/tierceron/atrium/speculatio/spiralis

all: certify devplugincarrier devplugintrcdb trcshell trcshellwin fenestra spiralis
