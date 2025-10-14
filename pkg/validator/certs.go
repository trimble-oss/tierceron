package validator

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/utils"
)

// Definition here: https://tools.ietf.org/html/rfc5280
type AlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier // OBJECT IDENTIFIER,
	Parameters asn1.RawValue         `asn1:"optional"` // ANY DEFINED BY algorithm OPTIONAL
}

// Definition here: https://tools.ietf.org/html/rfc2315
type DigestInfo struct {
	Algorithm AlgorithmIdentifier // DigestAlgorithmIdentifier
	Digest    []byte              // OCTET STRING
}

// Definition here: https://tools.ietf.org/html/rfc2315
type ContentInfo struct {
	ContentType asn1.ObjectIdentifier // OBJECT IDENTIFIER
	Content     asn1.RawValue         `asn1:"tag:0,explicit,optional"` // EXPLICIT ANY DEFINED BY contentType OPTIONAL
}

// Definition here: https://tools.ietf.org/html/rfc7292
type MacData struct {
	Mac        DigestInfo
	MacSalt    []byte // OCTET STRING
	Iterations int    `asn1:"optional,default:1"` // INTEGER DEFAULT 1
}

// Definition here: https://tools.ietf.org/html/rfc7292
type Pfx struct {
	Version  int // {v3(3)}(v3,...),
	AuthSafe ContentInfo
	MacData  MacData `asn1:"optional"`
}

// IsPfx verfies if this looks like a pfx.
func IsPfxRfc7292(byteCert []byte) (bool, error) {
	pfxStructure := new(Pfx)

	_, err := asn1.Unmarshal(byteCert, pfxStructure)
	if err != nil {
		return false, errors.New("failed to parse certificate pfx")
	}

	return true, nil
}

// ValidateCertificate validates certificate pointed to by the path
func ValidateCertificate(certPath string, host string) (bool, error) {
	byteCert, err := os.ReadFile(certPath)
	if err != nil {
		return false, errors.New("failed to read file: " + err.Error())
	}
	return ValidateCertificateBytes(byteCert, host, false)
}

// ValidateCertificateBytes validates certificate bytes
func ValidateCertificateBytes(byteCert []byte, host string, selfSignedOk bool) (bool, error) {
	block, _ := pem.Decode(byteCert)
	if block == nil {
		return false, errors.New("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, errors.New("failed to parse certificate: " + err.Error())
	}

	isValid, err := VerifyCertificate(cert, host, selfSignedOk)
	return isValid, err
}

// Borrowed from https://github.com/fcjr/aia-transport-go
// MIT License
func getCert(url string) (*x509.Certificate, error) {
	resp, err := http.Get(url)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(data)
}

func verifyCertHelper(cert *x509.Certificate, host string) (bool, error) {
	if err := cert.VerifyHostname(host); err != nil {
		return false, fmt.Errorf("hostname verification failed: %v", err)
	}

	if cert.NotBefore.After(cert.NotAfter) {
		return false, fmt.Errorf("certificate validity period is invalid")
	}

	return true, nil
}

// VerifyCertificate
func VerifyCertificate(cert *x509.Certificate, host string, verifyBySystemCertPool bool) (bool, error) {
	opts := x509.VerifyOptions{
		DNSName:     host,
		CurrentTime: time.Now(),
	}

	if !utils.IsWindows() {
		if verifyBySystemCertPool {
			rootCAs, err := x509.SystemCertPool()
			if err != nil {
				return false, err
			}
			opts.Roots = rootCAs
			opts.Intermediates = x509.NewCertPool()
		} else {
			opts.Roots = x509.NewCertPool()
			opts.Intermediates = x509.NewCertPool()
		}
	}

	if verifyBySystemCertPool {
		if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
			return false, errors.New("certificate is outside valid range probably expired")
		}

		// First attempt: Try with system cert pool as-is
		if _, err := cert.Verify(opts); err == nil {
			return true, nil
		}

		if !utils.IsWindows() {
			// Second attempt: Add known intermediate certificates proactively
			// This helps with certificates that require specific intermediate CAs
			for _, supportedIssuer := range coreopts.BuildOptions.GetSupportedCertIssuers() {
				if issuer, issuerErr := getCert(supportedIssuer); issuerErr == nil {
					opts.Intermediates.AddCert(issuer)
				}
				// Continue adding other issuers even if one fails
			}

			// Try verification again with added intermediates
			if _, err := cert.Verify(opts); err == nil {
				return true, nil
			}

			// Third attempt: Try with more flexible options for edge cases
			// Create a copy of options with relaxed KeyUsages (but keep other security checks)
			flexOpts := opts
			flexOpts.KeyUsages = []x509.ExtKeyUsage{
				x509.ExtKeyUsageAny,
				x509.ExtKeyUsageServerAuth,
				x509.ExtKeyUsageClientAuth,
			}

			if _, err := cert.Verify(flexOpts); err == nil {
				return true, nil
			}

			return false, errors.New("failed to verify certificate after all attempts")
		} else {
			// Windows path - simpler fallback
			return verifyCertHelper(cert, host)
		}
	} else {
		return verifyCertHelper(cert, host)
	}
}
