package initlib

import (
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
	"log"
)

var engines = [...]string{
	"templates",
	"values",
	"super-secrets",
	"value-metrics",
	"verification",
}

//CreateEngines adds engines specified by the list 'engines'
func CreateEngines(v *sys.Vault, logger *log.Logger) {
	// Delete the kv path secreat first time (originally v1)
	v.DeleteKVPath("secret")
	for _, eng := range engines {
		v.CreateKVPath(eng, eng+" vault engine")
		logger.Printf("Created engine %s\n", eng)
	}
}
