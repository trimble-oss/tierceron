GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -a -ldflags '-w' tierceron/webapi/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install tierceron/webapi/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install tierceron/trcconfig
configdbplugin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/trc-vault-plugin tierceron/trcvault
configwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -o $(GOBIN)/trcconfig.exe trcconfig/trcconfig.go
configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/trcconfig.mac tierceron/trcconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install tierceron/trcinit
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/trcinit.mac tierceron/trcinit 
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install tierceron/trcinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install tierceron/trcx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/trcx.mac tierceron/trcx
xlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=linux GOARCH=amd64 go build -buildmode=c-shared -a -ldflags '-w' -o $(GOBIN)/nc.so tierceron/configlib
maclib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -buildmode=c-shared -o $(GOBIN)/nc.dylib tierceron/configlib
winlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -o $(GOBIN)/nc.dll tierceron/configlib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install tierceron/trcxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install tierceron/trcpub
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

