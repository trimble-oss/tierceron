GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -a -ldflags '-w' bitbucket.org/dexterchaney/whoville/webapi/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/webapi/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultconfig
configwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -o $(GOBIN)/vaultconfig.exe vaultconfig/vaultconfig.go
configmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/vaultconfig.mac bitbucket.org/dexterchaney/whoville/vaultconfig
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultinit
seedmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/vaultinit.mac bitbucket.org/dexterchaney/whoville/vaultinit 
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultx
xmac:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=amd64 go build -o $(GOBIN)/vaultx.mac bitbucket.org/dexterchaney/whoville/vaultx
lib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=linux GOARCH=amd64 go build -a -ldflags '-w' -o $(GOBIN)/nc.so bitbucket.org/dexterchaney/whoville/configlib
winlib:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -o $(GOBIN)/nc.dll bitbucket.org/dexterchaney/whoville/configlib
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultpub
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

