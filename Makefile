GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

apiprod:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install  -tags "trcname prod" -a -ldflags '-w' github.com/trimble-oss/tierceron/trcweb/apiRouter
api:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install   github.com/trimble-oss/tierceron/trcweb/apiRouter
config:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "azure memonly"  github.com/trimble-oss/tierceron/cmd/trcconfig
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
ctl:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go install  -tags "memonly tc" github.com/trimble-oss/tierceron/cmd/trcctl
gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/apinator/service.proto

all: api config seed x xlib pub sub
