package main

import (
	"fmt"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	trcinitbase "github.com/trimble-oss/tierceron/trcinitbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/utils/mlock"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.6")
	env := "local"
	addr := coreopts.GetVaultHostPort()
	trcinitbase.CommonMain(&env, &addr, nil)
}
