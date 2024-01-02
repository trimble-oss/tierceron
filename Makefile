GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install  -tags "trcname prod" -a -ldflags '-w' github.com/trimble-oss/tierceron/trcweb/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install   github.com/trimble-oss/tierceron/trcweb/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly"  github.com/trimble-oss/tierceron/cmd/trcconfig

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
harbingplugintrcdb: harbingplugintrcdbbuild harbingplugintrcdbsha

configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure" -o $(GOBIN)/trcconfig.mac github.com/trimble-oss/tierceron/cmd/trcconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcinit
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure" -o $(GOBIN)/trcinit.mac github.com/trimble-oss/tierceron/cmd/trcinit 
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmdp/trcinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure" -o $(GOBIN)/trcx.mac github.com/trimble-oss/tierceron/cmd/trcx
xlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=linux GOARCH=amd64 go build   -buildmode=c-shared -a -ldflags '-w' -tags "azure memonly" -o $(GOBIN)/nc.so github.com/trimble-oss/tierceron/zeroconfiglib
maclib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build   -buildmode=c-shared -tags "azure" -o $(GOBIN)/nc.dylib github.com/trimble-oss/tierceron/zeroconfiglib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmdp/trcxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcpub
sub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/cmd/trcsub
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


gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

all: api certify devplugintrcdb config seed x xlib pub sub
