GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -gcflags=-G=0 -tags "trcname prod" -a -ldflags '-w' tierceron/webapi/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0  tierceron/webapi/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0  tierceron/trcconfig

deploypluginbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags=-G=0 -tags "prod memonly" -o trcvault/deploy/target/trc-vault-deploy-plugin tierceron/trcvault/plugindeployer
deploypluginsha:
	sha256sum trcvault/deploy/target/trc-vault-deploy-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-deploy-plugin.sha256
deployplugin: deploypluginbuild deploypluginsha

configdbdevpluginbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build -gcflags=-G=0 -tags "testflow insecure" -o trcvault/deploy/target/trc-vault-plugin tierceron/trcvault/plugin
configdbdevpluginsha:
	sha256sum trcvault/deploy/target/trc-vault-plugin | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-plugin.sha256
configdbdevplugin: configdbdevpluginbuild configdbdevpluginsha

configdbprodpluginbuild:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags=-G=0 -tags "prod memonly" -o trcvault/deploy/target/trc-vault-plugin-prod tierceron/trcvault/plugin
configdbprodpluginsha:
	sha256sum trcvault/deploy/target/trc-vault-plugin-prod | cut -d' ' -f1 > trcvault/deploy/target/trc-vault-plugin-prod.sha256
configdbprodplugin: configdbprodpluginbuild configdbprodpluginsha

configdbplugin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 go build -gcflags=-G=0 -tags testflow -o $(GOBIN)/trc-vault-plugin tierceron/trcvault/plugin
configwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -gcflags=-G=0  -o $(GOBIN)/trcconfig.exe trcconfig/trcconfig.go
configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -gcflags=-G=0  -o $(GOBIN)/trcconfig.mac tierceron/trcconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0  tierceron/trcinit
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -gcflags=-G=0  -o $(GOBIN)/trcinit.mac tierceron/trcinit 
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0  tierceron/trcinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0  tierceron/trcx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -gcflags=-G=0  -o $(GOBIN)/trcx.mac tierceron/trcx
xlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=linux GOARCH=amd64 go build -gcflags=-G=0  -buildmode=c-shared -a -ldflags '-w' -o $(GOBIN)/nc.so tierceron/configlib
maclib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -gcflags=-G=0  -buildmode=c-shared -o $(GOBIN)/nc.dylib tierceron/configlib
winlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -gcflags=-G=0  -buildmode=c-shared -o $(GOBIN)/nc.dll tierceron/configlib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0  tierceron/trcxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0  tierceron/trcpub
sub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install -gcflags=-G=0 tierceron/trcsub
certify:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -gcflags=-G=0 -o $(GOBIN)/trcplgtool -tags "awsecr" tierceron/trcvault/trcplgtoolbase
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

all: api config configdbdevplugin config seed x xlib pub sub
