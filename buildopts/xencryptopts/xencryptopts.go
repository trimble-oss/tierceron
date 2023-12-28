package xencryptopts

import (
	"errors"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

//	"time"

func FieldValidator(fields string, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error {
	return errors.New("not implemented")
}

func SetEncryptionSecret(config *eUtils.DriverConfig) error {
	return errors.New("not implemented")
}

func GetEncrpytors(secSection map[string]map[string]map[string]string) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func CreateEncrpytedReadMap(encrypted string) map[string]interface{} {
	return nil
}

func FieldReader(encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string, decryption map[string]interface{}) error {
	return errors.New("not implemented")
}

func PromptUserForFields(fields string, encrypted string, encryption map[string]interface{}) (map[string]interface{}, map[string]interface{}, error) {
	return nil, nil, errors.New("not implemented")
}

func FieldReplacer(fieldMap map[string]interface{}, encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error {
	return errors.New("not implemented")
}
