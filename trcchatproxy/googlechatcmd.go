package main

import (
	"github.com/trimble-oss/tierceron/trcchatproxy/pubsub"
	"github.com/trimble-oss/tierceron/trcchatproxy/trcchat"
)

func main() {
	shutdown := make(chan bool)
	pubsub.CommonInit(true)
	trcchat.CommonInit()

	<-shutdown
}
