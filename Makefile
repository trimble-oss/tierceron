GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install  -tags "trcname prod" -a -ldflags '-w' tierceron/webapi/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install   tierceron/webapi/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly"  tierceron/trcconfig

devplugincarrierbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -tags "insecure awsecr" -o trcvault/deploy/target/trc-vault-carrier-plugin tierceron/trcvault/plugins/carrier
devplugincarriersha:
	sha256sum trcvault/deploy/target/trc-vault-carrier-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-carrier-plugin.sha256
devplugincarrier: devplugincarrierbuild devplugincarriersha

devplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build  -tags "insecure" -o trcvault/deploy/target/trc-vault-plugin tierceron/trcvault/plugins/trcdb
devplugintrcdbsha:
	sha256sum trcvault/deploy/target/trc-vault-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-plugin.sha256
devplugintrcdb: devplugintrcdbbuild devplugintrcdbsha

harbingplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build  -tags "insecure harbinger" -o trcvault/deploy/target/trc-vault-plugin tierceron/trcvault/plugins/trcdb
harbingplugintrcdbsha:
	sha256sum trcvault/deploy/target/trc-vault-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-plugin.sha256
harbingplugintrcdb: harbingplugintrcdbbuild harbingplugintrcdbsha

prodplugintrcdbbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -tags "prod awsecr memonly" -o trcvault/deploy/target/trc-vault-plugin-prod tierceron/trcvault/plugins/trcdb
prodplugintrcdbsha:
	sha256sum trcvault/deploy/target/trc-vault-plugin-prod | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-plugin-prod.sha256
prodplugintrcdb: prodplugintrcdbbuild prodplugintrcdbsha

pluginprodcarrierbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -tags "prod memonly" -o trcvault/deploy/target/trc-vault-carrier-plugin-prod tierceron/trcvault/plugins/carrier
pluginprodcarriersha:
	sha256sum trcvault/deploy/target/trc-vault-carrier-plugin-prod | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-carrier-plugin-prod.sha256
pluginprodcarrier: pluginprodcarrierbuild pluginprodcarriersha

configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure memonly" -o $(GOBIN)/trcconfig.mac tierceron/trcconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" tierceron/trcinit
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure memonly" -o $(GOBIN)/trcinit.mac tierceron/trcinit 
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" tierceron/trcinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" tierceron/trcx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build  -tags "azure memonly" -o $(GOBIN)/trcx.mac tierceron/trcx
xlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=linux GOARCH=amd64 go build   -buildmode=c-shared -a -ldflags '-w' -tags "azure memonly" -o $(GOBIN)/nc.so tierceron/configlib
maclib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build   -buildmode=c-shared -tags "azure memonly" -o $(GOBIN)/nc.dylib tierceron/configlib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" tierceron/trcxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" tierceron/trcpub
sub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly" tierceron/trcsub
certify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build  -o $(GOBIN)/trcplgtool -tags "memonly awsecr" tierceron/trcvault/trcplgtoolbase
agentctl: 
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "memonly" tierceron/trcagentctl
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

all: api devplugintrcdb config seed x xlib pub sub
