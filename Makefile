GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/apiRouter webapi/apiRouter/router.go
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/vaultconfig vaultconfig/vaultconfig.go
configwin:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=windows GOARCH=amd64 go build -o $(GOBIN)/vaultconfig.exe vaultconfig/vaultconfig.go
seed:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/vaultinit vaultinit/init.go
x:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/vaultx vaultx/vaultx.go
pub:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/vaultpub vaultpub/vaultpub.go
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

