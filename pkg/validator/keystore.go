package validator

import (
	"bufio"
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/pavlo-v-chernykh/keystore-go/v4"

	"github.com/youmark/pkcs8"
	pkcs "golang.org/x/crypto/pkcs12"
	"golang.org/x/crypto/ssh"
)

// Copied from pkcs12.go... why can't they just make these public.  Gr...
// PEM block types
const (
	certificateType   = "CERTIFICATE"
	privateKeyType    = "PRIVATE KEY"
	rsaPrivateKeyType = "RSA PRIVATE KEY"
)

func StoreKeystore(driverConfig *config.DriverConfig, trustStorePassword string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	keystoreWriter := bufio.NewWriter(buffer)

	if driverConfig.KeyStore == nil {
		return nil, errors.New("cert bundle not properly named")
	}

	aliases := driverConfig.KeyStore.Aliases()
	sort.Strings(aliases)
	certChainsByBaseName := make(map[string][]keystore.Certificate)
	for _, alias := range aliases {
		if driverConfig.KeyStore.IsTrustedCertificateEntry(alias) {
			if tce, err := driverConfig.KeyStore.GetTrustedCertificateEntry(alias); err == nil {
				base := strings.TrimSuffix(path.Base(alias), path.Ext(path.Base(alias)))
				certChainsByBaseName[base] = append(certChainsByBaseName[base], tce.Certificate)
			}
		}
	}
	for _, alias := range aliases {
		if driverConfig.KeyStore.IsPrivateKeyEntry(alias) {
			if pke, err := driverConfig.KeyStore.GetPrivateKeyEntry(alias, []byte{}); err == nil {
				if len(pke.CertificateChain) == 0 {
					keyBase := strings.TrimSuffix(path.Base(alias), path.Ext(path.Base(alias)))
					certBase := strings.Replace(keyBase, "key", "cert", 1)
					if chain, ok := certChainsByBaseName[certBase]; ok {
						pke.CertificateChain = chain
					}
				}
				driverConfig.KeyStore.SetPrivateKeyEntry(alias, pke, []byte(trustStorePassword)) //nolint: errcheck
			}
		}
	}
	// Remove the #N staging entries — only the base alias (leaf cert) should remain as a trusted entry.
	for _, alias := range aliases {
		if driverConfig.KeyStore.IsTrustedCertificateEntry(alias) && strings.Contains(path.Base(alias), "#") {
			driverConfig.KeyStore.DeleteEntry(alias)
		}
	}

	storeErr := driverConfig.KeyStore.Store(keystoreWriter, []byte(trustStorePassword))
	if storeErr != nil {
		return nil, storeErr
	}
	keystoreWriter.Flush()

	return buffer.Bytes(), nil
}

func AddToKeystore(driverConfig *config.DriverConfig, alias string, password []byte, certBundleJks string, data []byte) error {
	// TODO: Add support for this format?  golang.org/x/crypto/pkcs12

	if !strings.HasSuffix(driverConfig.WantKeystore, ".jks") && strings.HasSuffix(certBundleJks, ".jks") {
		driverConfig.WantKeystore = certBundleJks
	}

	if driverConfig.KeyStore == nil {
		fmt.Fprintln(os.Stderr, "Making new keystore.")
		ks := keystore.New()
		driverConfig.KeyStore = &ks
	}

	block, _ := pem.Decode(data)
	if block == nil {
		key, cert, err := pkcs.Decode(data, string(password)) // Note the order of the return values.
		if err != nil {
			return err
		}
		pkcs8Key, err := pkcs8.ConvertPrivateKeyToPKCS8(key, password)
		if err != nil {
			return err
		}

		driverConfig.KeyStore.SetPrivateKeyEntry(alias, keystore.PrivateKeyEntry{
			CreationTime: time.Now(),
			PrivateKey:   pkcs8Key,
			CertificateChain: []keystore.Certificate{
				{
					Type:    "X509",
					Content: cert.Raw,
				},
			},
		}, password)

	} else {
		if block.Type == certificateType {
			aliasCommon := strings.Replace(alias, "cert.pem", "", 1)
			rest := data
			chainIdx := 0
			for {
				var b *pem.Block
				b, rest = pem.Decode(rest)
				if b == nil {
					break
				}
				if b.Type != certificateType {
					continue
				}
				entryAlias := aliasCommon
				if chainIdx > 0 {
					entryAlias = fmt.Sprintf("%s#%d", aliasCommon, chainIdx)
				}
				driverConfig.KeyStore.SetTrustedCertificateEntry(entryAlias, keystore.TrustedCertificateEntry{
					CreationTime: time.Now(),
					Certificate: keystore.Certificate{
						Type:    "X509",
						Content: b.Bytes,
					},
				})
				chainIdx++
			}
			return nil
		}
		if block.Type == privateKeyType || block.Type == rsaPrivateKeyType {
			var pkcs8KeyBytes []byte
			if block.Type == privateKeyType {
				pkcs8KeyBytes = block.Bytes
			} else {
				rsaKey, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes)
				if parseErr != nil {
					return parseErr
				}
				var marshalErr error
				pkcs8KeyBytes, marshalErr = x509.MarshalPKCS8PrivateKey(rsaKey)
				if marshalErr != nil {
					return marshalErr
				}
			}
			aliasCommon := strings.Replace(alias, "key.pem", "", 1)
			driverConfig.KeyStore.SetPrivateKeyEntry(aliasCommon, keystore.PrivateKeyEntry{
				CreationTime: time.Now(),
				PrivateKey:   pkcs8KeyBytes,
			}, password)
			return nil
		}
		privateKeyBytes, err := ssh.ParseRawPrivateKey(data)
		if err == nil {
			privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKeyBytes)
			if err != nil {
				return err
			}
			aliasCommon := strings.Replace(alias, "key.pem", "", 1)

			driverConfig.KeyStore.SetPrivateKeyEntry(aliasCommon, keystore.PrivateKeyEntry{
				CreationTime: time.Now(),
				PrivateKey:   privateKeyBytes,
			}, password)
		} else {
			return err
		}
	}

	return nil
}

// ValidateKeyStore validates the sendgrid API key.
func ValidateKeyStore(config *coreconfig.CoreConfig, filename string, pass string) (bool, error) {
	file, err := os.ReadFile(filename)
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

		switch (*pemBlock).Type {
		case certificateType:
			var cert x509.Certificate
			_, errUnmarshal := asn1.Unmarshal((*pemBlock).Bytes, &cert)
			if errUnmarshal != nil {
				return false, errors.New("failed to parse: " + err.Error())
			}

			isCertValid, err := VerifyCertificate(&cert, "", true)
			if err != nil {
				eUtils.LogInfo(config, "Certificate validation failure.")
			}
			isValid = isCertValid
		case privateKeyType:
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
