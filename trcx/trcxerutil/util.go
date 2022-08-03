package trcxerutil

import (
	"errors"
	"fmt"
	"strings"
)

func PromptUserForFields(fields string, encrypted string) (map[string]interface{}, map[string]interface{}, error) {
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
			//input = "user"
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
		//input = "password"
		//validateInput = "password"
		if validateInput != input {
			return nil, nil, errors.New("Entered values for '" + encryptedField + "' do not match, exiting...")
		}
		encryptedMap[encryptedField] = input
	}

	return fieldMap, encryptedMap, nil
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
				valSection["super-secrets"][valueSectionMap][field] = valueField.(string)
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
				valSection["super-secrets"][valueSectionMap][field] = valueField.(string)
				continue
			}
		}
	}

	return nil
}
