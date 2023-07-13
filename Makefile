GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install  -tags "trcname prod" -a -ldflags '-w' github.com/trimble-oss/tierceron/webapi/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install   github.com/trimble-oss/tierceron/webapi/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly"  github.com/trimble-oss/tierceron/trcconfig

devplugincarrierbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -tags "insecure awsecr" -o trcvault/deploy/target/trc-vault-carrier-plugin github.com/trimble-oss/tierceron/trcvault/plugins/carrier
devplugincarriersha:
	sha256sum trcvault/deploy/target/trc-vault-carrier-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-carrier-plugin.sha256
devplugincarrier: devplugincarrierbuild devplugincarriersha

devplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build  -tags "insecure" -o trcvault/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/trcvault/plugins/trcdb
devplugintrcdbsha:
	sha256sum trcvault/deploy/target/trc-vault-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-plugin.sha256
devplugintrcdb: devplugintrcdbbuild devplugintrcdbsha

harbingplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build  -tags "insecure harbinger" -o trcvault/deploy/target/trc-vault-plugin github.com/trimble-oss/tierceron/trcvault/plugins/trcdb
harbingplugintrcdbsha:
	sha256sum trcvault/deploy/target/trc-vault-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-plugin.sha256
harbingplugintrcdb: harbingplugintrcdbbuild harbingplugintrcdbsha

configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure" -o $(GOBIN)/trcconfig.mac github.com/trimble-oss/tierceron/trcconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/trcinit
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure" -o $(GOBIN)/trcinit.mac github.com/trimble-oss/tierceron/trcinit 
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/trcinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/trcx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure" -o $(GOBIN)/trcx.mac github.com/trimble-oss/tierceron/trcx
xlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=linux GOARCH=amd64 go build   -buildmode=c-shared -a -ldflags '-w' -tags "azure memonly" -o $(GOBIN)/nc.so github.com/trimble-oss/tierceron/configlib
maclib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build   -buildmode=c-shared -tags "azure" -o $(GOBIN)/nc.dylib github.com/trimble-oss/tierceron/configlib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/trcxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/trcpub
sub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" github.com/trimble-oss/tierceron/trcsub
certify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/trcplgtool -tags "memonly awsecr" github.com/trimble-oss/tierceron/trcvault/trcplgtoolbase
trcshell: 
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/trcsh -tags "memonly" github.com/trimble-oss/tierceron/trcsh
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

all: api devplugintrcdb config seed x xlib pub sub
