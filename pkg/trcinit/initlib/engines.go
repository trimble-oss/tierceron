package initlib

import (
	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
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
func CreateEngines(config *core.CoreConfig, v *sys.Vault) {
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
