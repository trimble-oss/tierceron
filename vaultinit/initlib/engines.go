package initlib

import (
	"bitbucket.org/dexterchaney/whoville/utils"
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
	"log"
)

var engines = [...]string{
	"apiLogins",
	"templates",
	"values",
	"super-secrets",
	"value-metrics",
	"verification",
}

//CreateEngines adds engines specified by the list 'engines'
func CreateEngines(v *sys.Vault, logger *log.Logger) {
	// Delete the kv path secreat first time (originally v1)
	for _, eng := range engines {
		err := v.CreateKVPath(eng, eng+" vault engine")
		if err != nil {
			utils.LogErrorObject(err, logger, false)
			continue
		}
		logger.Printf("Created engine %s\n", eng)
	}
}
