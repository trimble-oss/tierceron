package validator

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	eUtils "tierceron/utils"
	"time"

	"github.com/lwithers/minijks/jks"

	pkcs "golang.org/x/crypto/pkcs12"
)

// Copied from pkcs12.go... why can't they just make these public.  Gr...
// PEM block types
const (
	certificateType = "CERTIFICATE"
	privateKeyType  = "PRIVATE KEY"
)

func PackKeystore(config *eUtils.DriverConfig) ([]byte, error) {
	return config.KeyStore.Pack(&jks.Options{
		Password:     "",
		KeyPasswords: make(map[string]string),
	})
}

func AddToKeystore(config *eUtils.DriverConfig, alias string, certRaw []byte) error {
	var pemPackerr error

	// For now, only supporting passwordless.
	block, _ := pem.Decode(certRaw)
	if block == nil {
		return errors.New("Not a pem.")
	}
	var kp *jks.Keypair
	var kc *jks.Cert

	switch block.Type {
	case "RSA PRIVATE KEY":
		kp = &jks.Keypair{
			Alias: alias,
		}
		kp.Timestamp = time.Now()
		kp.PrivateKey, pemPackerr = x509.ParsePKCS1PrivateKey(block.Bytes)

	case "EC PRIVATE KEY":
		kp = &jks.Keypair{
			Alias: alias,
		}
		kp.Timestamp = time.Now()
		kp.PrivateKey, pemPackerr = x509.ParseECPrivateKey(block.Bytes)

	case "CERTIFICATE":
		kc = &jks.Cert{
			Alias:     alias,
			Timestamp: time.Now(),
		}
		kc.Cert, pemPackerr = x509.ParseCertificate(block.Bytes)
	default:
		pemPackerr = fmt.Errorf("%s: unknown private key type %s",
			alias, block.Type)
	}
	if pemPackerr != nil {
		return pemPackerr
	}

	if kc != nil {
		config.KeyStore.Certs = append(config.KeyStore.Certs, kc)
	} else if kp != nil {
		config.KeyStore.Keypairs = append(config.KeyStore.Keypairs, kp)
	}

	return nil
}

// ValidateKeyStore validates the sendgrid API key.
func ValidateKeyStore(config *eUtils.DriverConfig, filename string, pass string) (bool, error) {
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

			isCertValid, err := VerifyCertificate(&cert, "")
			if err != nil {
				eUtils.LogInfo(config, "Certificate validation failure.")
			}
			isValid = isCertValid
		} else if (*pemBlock).Type == privateKeyType {
			var key rsa.PrivateKey
			_, errUnmarshal := asn1.Unmarshal((*pemBlock).Bytes, &key)
			if errUnmarshal != nil {
				return false, errors.New("failed to parse: " + err.Error())
			}

			if err := key.Validate(); err != nil {
				eUtils.LogInfo(config, "key validation didn't work")
			}
		}
	}

	return isValid, err
}
