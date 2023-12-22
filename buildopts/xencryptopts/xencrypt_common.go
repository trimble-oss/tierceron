//go:build !tc
// +build !tc

package xencryptopts

import (
	"errors"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

//	"time"

func FieldValidator(fieldName string, secretCombinedSection map[string]map[string]map[string]string, valueCombinedSection map[string]map[string]map[string]string) error {
	return errors.New("not implemented")
}

func SetEncryptionSecret(config *eUtils.DriverConfig) error {
	return errors.New("not implemented")
}

func GetEncrpytors(secretCombinedSection map[string]map[string]map[string]string) (string, error) {
	return "", errors.New("not implemented")
}

func CreateEncrpytedReadMap(field string) map[string]interface{} {
	return nil
}

func FieldReader(encryptorReadMap map[string]interface{}, secretCombinedSection map[string]map[string]map[string]string, valueCombinedSection map[string]map[string]map[string]string, encryption string) {
	return
}

func PromptUserForFields(fieldOne string, fieldTwo string, encryption string) (map[string]interface{}, map[string]interface{}, error) {
	return nil, nil, errors.New("not implemented")
}

func FieldReplacer(fieldChangedMap map[string]interface{}, encryptedChangedMap map[string]interface{}, secretCombinedSection map[string]map[string]map[string]string, valueCombinedSection map[string]map[string]map[string]string) {
}
