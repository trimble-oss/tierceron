package validator

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/trimble-oss/tierceron/utils"
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
	return ValidateCertificateBytes(byteCert, host)
}

// ValidateCertificateBytes validates certificate bytes
func ValidateCertificateBytes(byteCert []byte, host string) (bool, error) {
	block, _ := pem.Decode(byteCert)
	if block == nil {
		return false, errors.New("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, errors.New("failed to parse certificate: " + err.Error())
	}

	isValid, err := VerifyCertificate(cert, host)
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

// VerifyCertificate
func VerifyCertificate(cert *x509.Certificate, host string) (bool, error) {
	opts := x509.VerifyOptions{
		DNSName:     host,
		CurrentTime: time.Now(),
	}

	if !utils.IsWindows() {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return false, err
		}
		opts.Roots = rootCAs
		opts.Intermediates = x509.NewCertPool()
	}

	if _, err := cert.Verify(opts); err != nil {
		if !utils.IsWindows() {
			if _, ok := err.(x509.UnknownAuthorityError); ok {
				issuer, issuerErr := getCert("http://r3.i.lencr.org/")
				if issuerErr != nil {
					return false, issuerErr
				}
				opts.Intermediates.AddCert(issuer)
				if _, err := cert.Verify(opts); err != nil {
					return false, err
				} else {
					return true, nil
				}
			}
		}
		return false, errors.New("failed to verify certificate: " + err.Error())
	}
	return true, nil
}
