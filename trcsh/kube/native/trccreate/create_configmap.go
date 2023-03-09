package trccreate

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/util/validation"
)

var utf8bom = []byte{0xEF, 0xBB, 0xBF}

// validateNewConfigMap checks whether the keyname is valid
// Note, the rules for ConfigMap keys are the exact same as the ones for SecretKeys.
func validateNewConfigMap(configMap *corev1.ConfigMap, keyName string) error {
	if errs := validation.IsConfigMapKey(keyName); len(errs) > 0 {
		return fmt.Errorf("%q is not a valid key name for a ConfigMap: %s", keyName, strings.Join(errs, ","))
	}
	if _, exists := configMap.Data[keyName]; exists {
		return fmt.Errorf("cannot add key %q, another key by that name already exists in Data for ConfigMap %q", keyName, configMap.Name)
	}
	if _, exists := configMap.BinaryData[keyName]; exists {
		return fmt.Errorf("cannot add key %q, another key by that name already exists in BinaryData for ConfigMap %q", keyName, configMap.Name)
	}

	return nil
}

func addKeyFromLiteralToConfigMap(configMap *corev1.ConfigMap, keyName, data string) error {
	err := validateNewConfigMap(configMap, keyName)
	if err != nil {
		return err
	}
	configMap.Data[keyName] = data

	return nil
}

// Function from kubectl
func processEnvFileLine(line []byte, filePath string,
	currentLine int) (key, value string, err error) {

	if !utf8.Valid(line) {
		return ``, ``, fmt.Errorf("env file %s contains invalid utf8 bytes at line %d: %v",
			filePath, currentLine+1, line)
	}

	// We trim UTF8 BOM from the first line of the file but no others
	if currentLine == 0 {
		line = bytes.TrimPrefix(line, utf8bom)
	}

	// trim the line from all leading whitespace first
	line = bytes.TrimLeftFunc(line, unicode.IsSpace)

	// If the line is empty or a comment, we return a blank key/value pair.
	if len(line) == 0 || line[0] == '#' {
		return ``, ``, nil
	}

	data := strings.SplitN(string(line), "=", 2)
	key = data[0]
	if errs := validation.IsEnvVarName(key); len(errs) != 0 {
		return ``, ``, fmt.Errorf("%q is not a valid key name: %s", key, strings.Join(errs, ";"))
	}

	if len(data) == 2 {
		value = data[1]
	} else {
		// No value (no `=` in the line) is a signal to obtain the value
		// from the environment.
		value = os.Getenv(key)
	}
	return
}

// AddFromEnvFile processes an env file allows a generic addTo to handle the
// collection of key value pairs or returns an error.
func AddFromMemEnvFile(filePath string, data []byte, addTo func(key, value string) error) error {

	reader := bytes.NewReader(data)
	scanner := bufio.NewScanner(reader)
	currentLine := 0
	for scanner.Scan() {
		// Process the current line, retrieving a key/value pair if
		// possible.
		scannedBytes := scanner.Bytes()
		key, value, err := processEnvFileLine(scannedBytes, filePath, currentLine)
		if err != nil {
			return err
		}
		currentLine++

		if len(key) == 0 {
			// no key means line was empty or a comment
			continue
		}

		if err = addTo(key, value); err != nil {
			return err
		}
	}
	return nil
}

func HandleConfigMapFromEnvFileSource(configMap *corev1.ConfigMap, filePath string, data []byte) error {
	err := AddFromMemEnvFile(filePath, data, func(key, value string) error {
		return addKeyFromLiteralToConfigMap(configMap, key, value)
	})
	if err != nil {
		return err
	}

	return nil
}
