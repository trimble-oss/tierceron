package pluginutil

import (
	"errors"

	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func GetPluginCertifyMap(mod *kv.Modifier, pluginConfig map[string]interface{}) (map[string]interface{}, error) {
	if pluginName, ok := pluginConfig["pluginName"].(string); ok && pluginName != "" {
		certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/" + pluginName + "/Certify")
		if err != nil {
			return nil, err
		}
		return certifyMap, nil
	}
	return nil, errors.New("missing plugin name for configuration")
}
