package validator

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"io/ioutil"

	pkcs "golang.org/x/crypto/pkcs12"
)

// Copied from pkcs12.go... why can't they just make these public.  Gr...
// PEM block types
const (
	certificateType = "CERTIFICATE"
	privateKeyType  = "PRIVATE KEY"
)

//ValidateKeyStore validates the sendgrid API key.
func ValidateKeyStore(filename string, pass string) (bool, error) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, err
	}
	pemBlocks, errToPEM := pkcs.ToPEM(file, pass)
	if errToPEM != nil {
		return false, errors.New("failed to parse: " + err.Error())
	}
	isValid := false

	for _, pemBlock := range pemBlocks {
		// PEM constancts defined but not exposed in
		//	certificateType = "CERTIFICATE"
		//	privateKeyType  = "PRIVATE KEY"

		if (*pemBlock).Type == certificateType {
			var cert x509.Certificate
			_, errUnmarshal := asn1.Unmarshal((*pemBlock).Bytes, &cert)
			if errUnmarshal != nil {
				return false, errors.New("failed to parse: " + err.Error())
			}

			isCertValid, err := VerifyCertificate(&cert)
			if err != nil {
				fmt.Println("Certificate validation failure.")
			}
			isValid = isCertValid
		} else if (*pemBlock).Type == privateKeyType {
			var key rsa.PrivateKey
			_, errUnmarshal := asn1.Unmarshal((*pemBlock).Bytes, &key)
			if errUnmarshal != nil {
				return false, errors.New("failed to parse: " + err.Error())
			}

			if err := key.Validate(); err != nil {
				fmt.Println("key validation didn't work")
			}
		}
	}

	return isValid, err
}
