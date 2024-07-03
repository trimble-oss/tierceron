package xencryptopts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

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

var encryptSecret = ""

func SetEncryptionSecret(driverConfig *eUtils.DriverConfig) error {
	var encryptionSecretField = "encryptionSecret"
	if len(driverConfig.Trcxe) > 2 {
		var input, validateInput string
		fmt.Printf("Enter desired value for '%s': \n", encryptionSecretField)
		fmt.Scanln(&input)
		fmt.Printf("Re-enter desired value for '%s': \n", encryptionSecretField)
		fmt.Scanln(&validateInput)
		if validateInput != input {
			return errors.New("Entered values for '" + encryptionSecretField + "' do not match, exiting...")
		}
		encryptSecret = input
	} else {
		mod, modErr := helperkv.NewModifier(driverConfig.Insecure, driverConfig.Token, driverConfig.VaultAddress, driverConfig.Env, driverConfig.Regions, true, driverConfig.CoreConfig.Log)
		if modErr != nil {
			eUtils.LogErrorObject(&driverConfig.CoreConfig, modErr, false)
		}
		mod.Env = strings.Split(driverConfig.Env, "_")[0]
		data, readErr := xencryptopts.BuildOptions.LoadSecretFromSecretStore(mod)
		if readErr != nil {
			return readErr
		}
		if data == nil {
			return errors.New("Encryption secret could not be found.")
		}

		if encrypSec, ok := data["encryptionSecret"].(string); ok && encrypSec != "" {
			encryptSecret = encrypSec
		}
	}
	return nil
}

func GetEncryptors(secSection map[string]map[string]map[string]string) (map[string]interface{}, error) {
	encryption := map[string]interface{}{}
	encryptionList := []string{"salt", "initial_value"}
	for _, encryptionField := range encryptionList {
		for secretSectionMap := range secSection["super-secrets"] {
			if value, ok := secSection["super-secrets"][secretSectionMap][encryptionField]; ok {
				if value != "" {
					encryption[encryptionField] = value
				}
			}
		}
	}

	if ok, ok1 := encryption["salt"], encryption["initial_value"]; ok == nil || ok1 == nil {
		return nil, errors.New("could not find encryption values")
	}

	return encryption, nil
}

func CreateEncryptedReadMap(encryptedKeys string) map[string]interface{} {
	encryptedMap := map[string]interface{}{}
	encryptedKeysSplit := strings.Split(encryptedKeys, ",")

	for _, encryptedField := range encryptedKeysSplit {
		encryptedMap[encryptedField] = ""
	}

	return encryptedMap
}

func FieldReader(encryptedMap map[string]interface{}, secSection map[string]map[string]map[string]string, valSection map[string]map[string]map[string]string, decryption map[string]interface{}) error {
	for field, _ := range encryptedMap {
		found := false
		for secretSectionMap := range secSection["super-secrets"] {
			if secretVal, ok := secSection["super-secrets"][secretSectionMap][field]; ok {
				decryptedVal, decryptErr := xencryptopts.BuildOptions.Decrypt(secretVal, decryption)
				if decryptErr != nil {
					return decryptErr
				}
				fmt.Printf("field: %s value: %s \n", field, decryptedVal)
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
				decryptedVal, decryptErr := xencryptopts.BuildOptions.Decrypt(valueVal, decryption)
				if decryptErr != nil {
					return decryptErr
				}
				fmt.Printf("field: %s value: %s \n", field, decryptedVal)
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

	salt, iv, newEncryptErr := xencryptopts.BuildOptions.MakeNewEncryption()
	if newEncryptErr != nil {
		return nil, nil, newEncryptErr
	}

	encryption["salt"] = salt
	encryption["initial_value"] = iv

	for _, encryptedField := range encryptedSplit {
		var input string
		var validateInput string
		fmt.Printf("Enter desired value for '%s': \n", encryptedField)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input = scanner.Text()
		fmt.Printf("Re-enter desired value for '%s': \n", encryptedField)
		scanner.Scan()
		validateInput = scanner.Text()
		if validateInput != input {
			return nil, nil, errors.New("Entered values for '" + encryptedField + "' do not match, exiting...")
		}
		encryptedInput, encryptError := xencryptopts.BuildOptions.Encrypt(input, encryption)
		if encryptError != nil {
			return nil, nil, encryptError
		}
		encryptedMap[encryptedField] = encryptedInput
	}

	encryptedMap["salt"] = salt
	encryptedMap["initial_value"] = iv

	return fieldMap, encryptedMap, nil
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
