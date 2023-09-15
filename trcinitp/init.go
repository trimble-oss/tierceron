package main

import (
	"fmt"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	trcinitbase "github.com/trimble-oss/tierceron/trcinitbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	fmt.Println("Version: " + "1.6")
	env := "local"
	addr := coreopts.GetVaultHostPort()
	trcinitbase.CommonMain(&env, &addr, nil)
}
