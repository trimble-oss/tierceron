package main

import (
	"fmt"

	plgt "tierceron/trcvault/trcplgtoolbase"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	fmt.Println("Version: " + "1.01")
	plgt.PluginMain()
}
