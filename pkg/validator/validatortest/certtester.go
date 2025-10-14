package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/validator"
)

func main() {
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())

	if len(os.Args) != 4 {
		fmt.Printf("Usage: %s <cert.pem> <key.pem> <domain>\n", os.Args[0])
		os.Exit(1)
	}
	certPath := os.Args[1]
	keyPath := os.Args[2]
	domain := os.Args[3]

	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		fmt.Printf("Couldn't read cert file: %v\n", err)
		os.Exit(1)
	}
	keyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		fmt.Printf("Couldn't read key file: %v\n", err)
		os.Exit(1)
	}

	// Parse key pair
	certPair, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		fmt.Printf("Couldn't construct key pair: %v\n", err)
		os.Exit(1)
	}

	// Parse certificate for details
	block, _ := pem.Decode(certBytes)
	if block == nil {
		fmt.Println("Failed to decode PEM block")
		os.Exit(1)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Printf("Couldn't parse certificate: %v\n", err)
		os.Exit(1)
	}

	isValid, err := validator.VerifyCertificate(cert, domain, true)
	fmt.Println("Certificate loaded successfully!")
	fmt.Printf("Certificate verification result: ")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if isValid {
		fmt.Printf("  Status: Valid\n")
	} else {
		fmt.Printf("  Status: Invalid\n")
	}
	fmt.Printf("  Subject: %s\n", cert.Subject)
	fmt.Printf("  Issuer: %s\n", cert.Issuer)
	fmt.Println("Issuer URLs:", cert.IssuingCertificateURL)
	fmt.Printf("  Not Before: %s\n", cert.NotBefore)
	fmt.Printf("  Not After:  %s\n", cert.NotAfter)

	// Optionally, check if the private key matches the certificate
	if len(certPair.Certificate) > 0 {
		if isValid {
			fmt.Println("Key pair appears valid and matches the certificate.")
		} else {
			fmt.Println("Key pair matches the certificate, but is not valid.  Possible domain mismatch or other issue.")
		}
	} else {
		fmt.Println("Key pair does not match the certificate.")
	}
}
