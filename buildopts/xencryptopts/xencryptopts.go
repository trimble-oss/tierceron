package xencryptopts

import (
	"errors"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

// MakeNewEncryption is a function that returns a new encryption key and a new encryption salt
func MakeNewEncryption() (string, string, error) {
	return "", "", errors.New("not implemented")
}

// LoadSecretFromSecretStore is a function that loads a secret from the secret store.  It returns a map with
// the secret contained in the attribute named "encryptionSecret".
func LoadSecretFromSecretStore(mod *helperkv.Modifier) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

// Encrypt is a function accepts and input string to be encoded and an encryption map.  The map should contain
// the base64 encoded attributes: "salt" and "initial_value".  These attributes are used to encrypt the input
// string.  The function returns the base64 encoded encrypted string.
func Encrypt(input string, encryption map[string]interface{}) (string, error) {
	return "", errors.New("not implemented")
}

// Decrypt is a function that accepts a base64 encoded encrypted string and a decryption map.  The map should
// contain the base64 encoded attributes: "salt" and "initial_value".  These attributes are used to decrypt the
// input string.  The function returns the decrypted string.
func Decrypt(passStr string, decryption map[string]interface{}) (string, error) {
	return "", errors.New("not implemented")
}
