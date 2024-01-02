GOPATH=~/workspace/go:$(shell pwd)/vendor:$(shell pwd)
GOBIN=$(shell pwd)/bin
GOFILES=$(wildcard *.go)

askflume:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/trcchatproxy -tags "insecure tc" github.com/trimble-oss/tierceron/atrium/vestibulum/trcchatproxy

askflumeserver:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o $(GOBIN)/trcchatserver -tags "insecure tc" github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow
