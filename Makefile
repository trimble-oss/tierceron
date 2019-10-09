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
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -o $(GOBIN)/vaultconfig.exe bitbucket.org/dexterchaney/whoville/vaultconfig/vaultconfig.go
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultinit
seedp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultinitp
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultx
xp:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultxp
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install bitbucket.org/dexterchaney/whoville/vaultpub
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

