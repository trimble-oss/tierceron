package echocore

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	ttsdk "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/trcshtalksdk"
)

type EchoBus struct {
	InChan  chan *ttsdk.DiagnosticRequest
	OutChan chan *ttsdk.DiagnosticResponse
}

// Buses will be indexed by environment
type EchoNetwork map[string]*EchoBus

var GlobalEchoNetwork *cmap.ConcurrentMap[string, *EchoBus]

func InitNetwork(envSlice []string) {
	echoNetwork := cmap.New[*EchoBus]()
	GlobalEchoNetwork = &echoNetwork

	for _, env := range envSlice {
		GlobalEchoNetwork.Set(env, new(EchoBus))
	}
}
