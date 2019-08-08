package validator

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"

	pkcs "bitbucket.org/dexterchaney/crypto/pkcs12"
)

//ValidateSendGrid validates the sendgrid API key.
func ValidateKeyStore(filename string, pass string) (bool, error) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, err
	}
	keys, certs, err := pkcs.DecodeAll(file, pass)

	fmt.Println(len(keys))
	fmt.Println(len(certs))
	if err != nil {
		return false, errors.New("failed to parse: " + err.Error())
	}

	if err := keys[0].(*rsa.PrivateKey).Validate(); err != nil {
		fmt.Println("key validation didn't work")
	}

	isValid, err := VerifyCertificate(certs[0])
	return isValid, err
}
