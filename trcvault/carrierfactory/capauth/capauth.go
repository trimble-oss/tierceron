package capauth

import (
	"log"
	"sync"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
)

var onceMemo sync.Once

func Init(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) error {

	certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/trcagentctl/Certify")
	if err != nil {
		return err
	}

	if _, ok := certifyMap["trcsha256"]; ok {
		logger.Println("Registering cap auth.")
		go func() {
			//			err := cap.Tap("/home/jrieke/workspace/Github/tierceron/trcagentctl/__debug_bin", certifyMap["trcsha256"].(string))
			err := cap.Tap("/home/azuredeploy/bin/trcagentctl", certifyMap["trcsha256"].(string))
			if err != nil {
				logger.Println("Cap tap failed with error: " + err.Error())
			}
		}()
		logger.Println("Memorizing")
		go MemorizeAndStart(pluginConfig, logger)
	}
	return nil
}

// Things to make available to trusted agent.
func MemorizeAndStart(memorizeFields map[string]interface{}, logger *log.Logger) {
	for key, value := range memorizeFields {
		switch key {
		case "vaddress", "pubrole", "configrole", "kubeconfig":
			logger.Println("Memorizing: " + key)
			cap.TapMemorize(key, value.(string))
		default:
			logger.Println("Skipping key: " + key)
		}
	}

	// TODO: make port configured and stored in vault.
	cap.TapServer("127.0.0.1:12384")
}
