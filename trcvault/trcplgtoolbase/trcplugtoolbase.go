package main

import (
	"fmt"

	plgt "tierceron/trcvault/trcplgtool"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	fmt.Println("Version: " + "1.01")
	plgt.PluginMain()
}
