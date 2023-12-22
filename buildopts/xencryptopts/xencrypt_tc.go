//go:build tc
// +build tc

package xencryptopts

import (
	trcxerutil "VaultConfig.TenantConfig/util/trcxerutil"
	eUtils "github.com/trimble-oss/tierceron/utils"
)

func FieldValidator(fields string, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error {
	return trcxerutil.FieldValidator(fieldName, secretCombinedSection, valueCombinedSection)
}

func SetEncryptionSecret(config *eUtils.DriverConfig) error {
	return trcxerutil.SetEncryptionSecret(config)
}

func GetEncrpytors(secSection map[string]map[string]map[string]string) (map[string]interface{}, error) {
	return trcxerutil.GetEncrpytors(secretCombinedSection)
}

func CreateEncrpytedReadMap(encrypted string) map[string]interface{} {
	return trcxerutil.CreateEncrpytedReadMap(encrypted)
}

func FieldReader(encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string, decryption map[string]interface{}) error {
	return trcxerutil.FieldReader(encryptorReadMap, secretCombinedSection, valueCombinedSection, encryption)
}

func PromptUserForFields(fields string, encrypted string, encryption map[string]interface{}) (map[string]interface{}, map[string]interface{}, error) {
	return trcxerutil.PromptUserForFields(fieldOne, fieldTwo, encryption)
}

func FieldReplacer(fieldMap map[string]interface{}, encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error {
	return trcxerutil.FieldReplacer(fieldChangedMap, encryptedChangedMap, secretCombinedSection, valueCombinedSection)
}
