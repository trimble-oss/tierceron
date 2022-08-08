package trcxerutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	"golang.org/x/crypto/pbkdf2"
)

var encryptSecret = ""

func SetEncryptionSecret(config *eUtils.DriverConfig) error {
	mod, modErr := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, config.Regions, config.Log)
	if modErr != nil {
		eUtils.LogErrorObject(config, modErr, false)
	}
	mod.Env = strings.Split(config.Env, "_")[0]
	data, readErr := mod.ReadData("super-secrets/Restricted/VaultDatabase/config")
	if readErr != nil {
		return readErr
	}
	if data == nil {
		return errors.New("Encryption secret could not be found.")
	}

	if encrypSec, ok := data["encryptionSecret"].(string); ok && encrypSec != "" {
		encryptSecret = encrypSec
	}
	return nil
}

func CreateEncrpytedReadMap(encrypted string) map[string]interface{} {
	encryptedMap := map[string]interface{}{}
	encryptedSplit := strings.Split(encrypted, ",")

	for _, encryptedField := range encryptedSplit {
		encryptedMap[encryptedField] = ""
	}

	return encryptedMap
}

func PromptUserForFields(fields string, encrypted string, encryption map[string]interface{}) (map[string]interface{}, map[string]interface{}, error) {
	fieldMap := map[string]interface{}{}
	encryptedMap := map[string]interface{}{}
	//Prompt user for desired value for fields
	//Prompt user for encrypted value 2x and validate they match.

	fieldSplit := strings.Split(fields, ",")
	encryptedSplit := strings.Split(encrypted, ",")

	for _, field := range fieldSplit {
		if !strings.Contains(encrypted, field) {
			var input string
			fmt.Printf("Enter desired value for '%s': \n", field)
			fmt.Scanln(&input)
			fieldMap[field] = input
		}
	}

	for _, encryptedField := range encryptedSplit {
		var input string
		var validateInput string
		fmt.Printf("Enter desired vaule for '%s': \n", encryptedField)
		fmt.Scanln(&input)
		fmt.Printf("Re-enter desired value for '%s': \n", encryptedField)
		fmt.Scanln(&validateInput)
		if validateInput != input {
			return nil, nil, errors.New("Entered values for '" + encryptedField + "' do not match, exiting...")
		}
		encryptedInput := Encrypt(input, encryption)
		encryptedMap[encryptedField] = encryptedInput
	}

	return fieldMap, encryptedMap, nil
}

func deriveKey(passphrase string, salt []byte) ([]byte, []byte) {
	return pbkdf2.Key([]byte(passphrase), salt, 1000, 32, sha256.New), salt
}

func Encrypt(input string, encryption map[string]interface{}) string {
	salt, _ := base64.StdEncoding.DecodeString(encryption["salt"].(string))
	iv, _ := base64.StdEncoding.DecodeString(encryption["initial_value"].(string))
	key, _ := deriveKey(encryptSecret, []byte(salt))
	b, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(b)
	data := aesgcm.Seal(nil, []byte(iv), []byte(input), nil)
	return base64.StdEncoding.EncodeToString(data)
}

func Decrypt(passStr string, decryption map[string]interface{}) string {
	salt, _ := base64.StdEncoding.DecodeString(decryption["salt"].(string))
	iv, _ := base64.StdEncoding.DecodeString(decryption["initial_value"].(string))
	data, _ := base64.StdEncoding.DecodeString(passStr)
	key, _ := deriveKey(encryptSecret, salt)
	b, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(b)
	data, _ = aesgcm.Open(nil, iv, data, nil)
	return string(data)
}

func GetEncrpytors(secSection map[string]map[string]map[string]string) (map[string]interface{}, error) {
	encrpytion := map[string]interface{}{}
	encrpytionList := []string{"salt", "initial_value"}
	for _, encryptionField := range encrpytionList {
		for secretSectionMap := range secSection["super-secrets"] {
			if value, ok := secSection["super-secrets"][secretSectionMap][encryptionField]; ok {
				if value != "" {
					encrpytion[encryptionField] = value
				}
			}
		}
	}

	if ok, ok1 := encrpytion["salt"], encrpytion["initial_value"]; ok == nil || ok1 == nil {
		return nil, errors.New("could not find encryption values")
	}

	return encrpytion, nil
}

func FieldValidator(fields string, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error {
	valueFields := strings.Split(fields, ",")
	valFieldMap := map[string]bool{}
	for _, valueField := range valueFields {
		valFieldMap[valueField] = false
	}
	for valueField, _ := range valFieldMap {
		for secretSectionMap := range secSection["super-secrets"] {
			if _, ok := secSection["super-secrets"][secretSectionMap][valueField]; ok {
				valFieldMap[valueField] = true
			}
		}

		for valueSection := range valSection["values"] {
			if _, ok := valSection["values"][valueSection][valueField]; ok {
				valFieldMap[valueField] = true
			}
		}
	}

	for valField, valFound := range valFieldMap {
		if !valFound {
			return errors.New("This field does not exist in this seed file: " + valField)
		}
	}

	return nil
}

func FieldReader(encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string, decryption map[string]interface{}) error {
	for field, _ := range encryptedMap {
		found := false
		for secretSectionMap := range secSection["super-secrets"] {
			if secretVal, ok := secSection["super-secrets"][secretSectionMap][field]; ok {
				fmt.Printf("field: %s value: %s \n", field, Decrypt(secretVal, decryption))
				//secSection["super-secrets"][secretSectionMap][field] = Decrypt(secretVal, decryption)
				found = true
				continue
			}
		}
		if found {
			continue
		}

		for valueSectionMap := range valSection["values"] {
			if valueVal, ok := valSection["values"][valueSectionMap][field]; ok {
				fmt.Printf("field: %s value: %s \n", field, Decrypt(valueVal, decryption))
				//valSection["values"][valueSectionMap][field] = Decrypt(valueVal, decryption)
				found = true
				continue
			}
		}
		if !found {
			return errors.New("Could not find encrypted field inside seed file.")
		}
	}
	return nil
}

func FieldReplacer(fieldMap map[string]interface{}, encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string) error {
	for field, valueField := range fieldMap {
		found := false
		for secretSectionMap := range secSection["super-secrets"] {
			if _, ok := secSection["super-secrets"][secretSectionMap][field]; ok {
				secSection["super-secrets"][secretSectionMap][field] = valueField.(string)
				found = true
				continue
			}
		}

		if found {
			continue
		}

		for valueSectionMap := range valSection["values"] {
			if _, ok := valSection["values"][valueSectionMap][field]; ok {
				valSection["values"][valueSectionMap][field] = valueField.(string)
				continue
			}
		}
	}

	for field, valueField := range encryptedMap {
		found := false
		for secretSectionMap := range secSection["super-secrets"] {
			if _, ok := secSection["super-secrets"][secretSectionMap][field]; ok {
				secSection["super-secrets"][secretSectionMap][field] = valueField.(string)
				found = true
				continue
			}
		}
		if found {
			continue
		}

		for valueSectionMap := range valSection["values"] {
			if _, ok := valSection["values"][valueSectionMap][field]; ok {
				valSection["values"][valueSectionMap][field] = valueField.(string)
				continue
			}
		}
	}

	return nil
}
