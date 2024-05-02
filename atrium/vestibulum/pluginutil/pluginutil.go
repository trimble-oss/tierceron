package pluginutil

import (
	"errors"

	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func GetPluginCertifyMap(mod *kv.Modifier, pluginConfig map[string]interface{}) (map[string]interface{}, error) {

	certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginConfig["pluginName"].(string) + "/Certify")
	if err != nil {
		return nil, err
	}
	return certifyMap, errors.New("missing plugin certification")
}
