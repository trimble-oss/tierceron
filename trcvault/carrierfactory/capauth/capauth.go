package capauth

import (
	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
)

func Init(mod *kv.Modifier) error {

	certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/trcagentctl/Certify")
	if err != nil {
		return err
	}
	// TODO: Get patch to agent...

	if _, ok := certifyMap["trcsha256"]; ok {
		cap.Tap("ExePath", "Exesha256")
	}
	// github.com/trimble-oss/tierceron-hat
	return nil
}
