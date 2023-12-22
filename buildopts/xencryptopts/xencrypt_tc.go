//go:build tc
// +build tc

package xencryptopts

import (
	trcxerutil "VaultConfig.TenantConfig/util/trcxerutil"
	eUtils "github.com/trimble-oss/tierceron/utils"
)

func FieldValidator(fieldName string, secretCombinedSection map[string]map[string]map[string]string, valueCombinedSection map[string]map[string]map[string]string) error {
	return trcxerutil.FieldValidator(fieldName, secretCombinedSection, valueCombinedSection)
}

func SetEncryptionSecret(config *eUtils.DriverConfig) error {
	return trcxerutil.SetEncryptionSecret(config)
}

func GetEncrpytors(secretCombinedSection map[string]map[string]map[string]string) (string, error) {
	return trcxerutil.GetEncrpytors(secretCombinedSection)
}

func CreateEncrpytedReadMap(field string) map[string]interface{} {
	return trcxerutil.CreateEncrpytedReadMap(config.Trcxe[1])
}

func FieldReader(encryptorReadMap map[string]interface{}, secretCombinedSection map[string]map[string]map[string]string, valueCombinedSection map[string]map[string]map[string]string, encryption string) {
	return trcxerutil.FieldReader(encryptorReadMap, secretCombinedSection, valueCombinedSection, encryption)
}

func PromptUserForFields(fieldOne string, fieldTwo string, encryption string) (map[string]interface{}, map[string]interface{}, error) {
	return trcxerutil.PromptUserForFields(fieldOne, fieldTwo, encryption)
}

func FieldReplacer(fieldChangedMap, encryptedChangedMap, secretCombinedSection, valueCombinedSection) {
	return trcxerutil.FieldReplacer(fieldChangedMap, encryptedChangedMap, secretCombinedSection, valueCombinedSection)
}
