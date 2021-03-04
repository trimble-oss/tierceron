GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -a -ldflags '-w' Vault.Whoville/webapi/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install Vault.Whoville/webapi/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install Vault.Whoville/vaultconfig
configwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -o $(GOBIN)/vaultconfig.exe vaultconfig/vaultconfig.go
configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/vaultconfig.mac Vault.Whoville/vaultconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install Vault.Whoville/vaultinit
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/vaultinit.mac Vault.Whoville/vaultinit 
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install Vault.Whoville/vaultinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install Vault.Whoville/vaultx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/vaultx.mac Vault.Whoville/vaultx
lib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=linux GOARCH=amd64 go build -buildmode=c-shared -a -ldflags '-w' -o $(GOBIN)/nc.so Vault.Whoville/configlib
maclib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -buildmode=c-shared -o $(GOBIN)/nc.dylib Vault.Whoville/configlib
winlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -o $(GOBIN)/nc.dll Vault.Whoville/configlib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install Vault.Whoville/vaultxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install Vault.Whoville/vaultpub
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

