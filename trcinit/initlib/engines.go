package initlib

import (
	eUtils "github.com/trimble-oss/tierceron/utils"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
)

var engines = [...]string{
	"apiLogins",
	"templates",
	"values",
	"super-secrets",
	"value-metrics",
	"verification",
}

// CreateEngines adds engines specified by the list 'engines'
func CreateEngines(config *eUtils.DriverConfig, v *sys.Vault) {
	// Delete the kv path secreat first time (originally v1)
	for _, eng := range engines {
		err := v.CreateKVPath(eng, eng+" vault engine")
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			continue
		}
		config.Log.Printf("Created engine %s\n", eng)
	}
}
