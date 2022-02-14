package utils

import (
	"log"
	helperkv "tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"
)

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultMod(config *DriverConfig) (*helperkv.Modifier, *sys.Vault, error) {
	vault, err := sys.NewVault(true, config.VaultAddress, config.Env, false, false, config.ExitOnFailure, config.Log)
	if err != nil {
		LogErrorObject(err, config.Log, false)
		return nil, nil, err
	}
	vault.SetToken(config.Token)

	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, config.Log)
	if err != nil {
		LogErrorObject(err, config.Log, false)
		return nil, nil, err
	}
	mod.Env = config.Env
	mod.Version = "0"
	mod.VersionFilter = config.VersionFilter

	return mod, vault, nil
}

// Helper to easiliy intialize a vault and a mod all at once.
func InitVaultModForPlugin(pluginConfig map[string]interface{}, logger *log.Logger) (*helperkv.Modifier, *sys.Vault, error) {
	exitOnFailure := false
	if _, ok := pluginConfig["exitOnFailure"]; ok {
		exitOnFailure = pluginConfig["exitOnFailure"].(bool)
	}
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
		ExitOnFailure:  exitOnFailure,
		Log:            logger,
	}

	return InitVaultMod(&config)
}
