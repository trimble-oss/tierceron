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

	clientOnly := len(os.Args) > 1 && os.Args[1] == "-client"
	if clientOnly && len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s -client <cert.pem> <host:port>\n", os.Args[0])
		os.Exit(1)
	} else if !clientOnly && len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <cert.pem> <key.pem> <domain>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "   or: %s -client <cert.pem> <host:port>\n", os.Args[0])
		os.Exit(1)
	}

	if clientOnly {
		certBytes, _ := ioutil.ReadFile(os.Args[2])
		block, _ := pem.Decode(certBytes)
		cert, _ := x509.ParseCertificate(block.Bytes)
		certPool := x509.NewCertPool()
		certPool.AddCert(cert)
		conn, err := tls.Dial("tcp", os.Args[3], &tls.Config{RootCAs: certPool})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Connection failed: %v\n", err)
			os.Exit(1)
		}
		conn.Close()
		fmt.Fprintln(os.Stderr, "Connection successful!")
		return
	}

	certPath := os.Args[1]
	keyPath := os.Args[2]
	domain := os.Args[3]

	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't read cert file: %v\n", err)
		os.Exit(1)
	}
	keyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't read key file: %v\n", err)
		os.Exit(1)
	}

	// Parse key pair
	certPair, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't construct key pair: %v\n", err)
		os.Exit(1)
	}

	// Parse certificate for details
	block, _ := pem.Decode(certBytes)
	if block == nil {
		fmt.Fprintln(os.Stderr, "Failed to decode PEM block")
		os.Exit(1)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't parse certificate: %v\n", err)
		os.Exit(1)
	}

	isValid, err := validator.VerifyCertificate(cert, domain, true)
	fmt.Fprintln(os.Stderr, "Certificate loaded successfully!")
	fmt.Fprintf(os.Stderr, "Certificate verification result: ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
	} else if isValid {
		fmt.Fprintf(os.Stderr, "  Status: Valid\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Status: Invalid\n")
	}
	fmt.Fprintf(os.Stderr, "  Subject: %s\n", cert.Subject)
	fmt.Fprintf(os.Stderr, "  Issuer: %s\n", cert.Issuer)
	fmt.Fprintln(os.Stderr, "Issuer URLs:", cert.IssuingCertificateURL)
	fmt.Fprintf(os.Stderr, "  Not Before: %s\n", cert.NotBefore)
	fmt.Fprintf(os.Stderr, "  Not After:  %s\n", cert.NotAfter)

	// Optionally, check if the private key matches the certificate
	if len(certPair.Certificate) > 0 {
		if isValid {
			fmt.Fprintln(os.Stderr, "Key pair appears valid and matches the certificate.")
		} else {
			fmt.Fprintln(os.Stderr, "Key pair matches the certificate, but is not valid.  Possible domain mismatch or other issue.")
		}
	} else {
		fmt.Fprintln(os.Stderr, "Key pair does not match the certificate.")
	}
}
