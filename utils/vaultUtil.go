package utils

import (
	"log"
	helperkv "tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"
)

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultMod(config *DriverConfig) (*helperkv.Modifier, *sys.Vault, error) {
	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, config.Log)
	mod.Env = config.Env
	mod.Version = "0"
	mod.VersionFilter = config.VersionFilter

	vault, err := sys.NewVault(true, config.VaultAddress, mod.Env, false, false, false, config.Log)
	if err != nil {
		LogErrorObject(err, config.Log, false)
		return nil, nil, err
	}
	vault.SetToken(config.Token)

	return mod, vault, nil
}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForPlugin(pluginConfig map[string]interface{}, logger *log.Logger) (*helperkv.Modifier, *sys.Vault, error) {
	config := DriverConfig{
		Insecure:       pluginConfig["insecure"].(bool),
		Token:          pluginConfig["token"].(string),
		VaultAddress:   pluginConfig["address"].(string),
		Env:            pluginConfig["env"].(string),
		Regions:        pluginConfig["regions"].([]string),
		SecretMode:     true, //  "Only override secret values in templates?"
		ServicesWanted: []string{},
		StartDir:       append([]string{}, ""),
		EndDir:         "",
		WantCerts:      false,
		GenAuth:        false,
		Log:            logger,
	}

	return InitVaultMod(&config)
}
