package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

//CertPath is the path to the cert file directory
const CertPath = "./certs/cert_files/"

//GenerateCerts generates a root cert, a root key, a child cert, and a child key. It then validates the root cert and returns the http client
func main() {
	//generate private key and write to .pem file
	privateKey, err := CreatePrivateKey("root_key.pem")
	if err != nil {
		panic(err)
	}
	//get public key
	publicKey := privateKey.Public()

	//create cert template
	rootCertTmpl, err := CertTemplate()
	if err != nil {
		panic(err)
	}
	rootCertTmpl.IsCA = true
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	rootCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}

	//create cert and write to .pem file
	rootCert, err := CreateCert(rootCertTmpl, rootCertTmpl, publicKey, privateKey, "root_cert.pem")
	if err != nil {
		panic(err)
	}

	servPrivateKey, err := CreatePrivateKey("serv_key.pem")
	if err != nil {
		panic(err)
	}
	//get public key
	servPublicKey := servPrivateKey.Public()

	servCertTmpl, err := CertTemplate()
	if err != nil {
		panic(err)
	}
	servCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	servCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	servCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}

	//create cert and write to .pem file
	_, err = CreateCert(servCertTmpl, rootCert, servPublicKey, privateKey, "serv_cert.pem")
	if err != nil {
		panic(err)
	}
}

//CertTemplate generates a random serial number
func CertTemplate() (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Viewpoint, Inc."},
		},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 3, 0), // valid for a day
		BasicConstraintsValid: true,
	}
	return &tmpl, nil
}

//CreatePrivateKey generates a private key and saves it to a .pem file
func CreatePrivateKey(fileName string) (privKey *rsa.PrivateKey, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	//encode private key
	pemPrivateBlock := &pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	path := CertPath + fileName
	//create new file for private key
	pemPrivateFile, err := os.Create(path)
	if err != nil {
		return privateKey, err
	}
	//write to file and close it
	err = pem.Encode(pemPrivateFile, pemPrivateBlock)
	if err != nil {
		return privateKey, err
	}
	pemPrivateFile.Close()
	fmt.Println("private key generated and written to", path)
	return privateKey, nil
}

//CreateCert creates a cert and saves it to a .pem file
func CreateCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}, fileName string) (cert *x509.Certificate, err error) {
	//cert *x509.Certificate,
	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return nil, err
	}

	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}

	pemCertBlock := &pem.Block{Type: "CERTIFICATE", Bytes: certDER}

	path := CertPath + fileName
	//create new file for private key
	pemCertFile, err := os.Create(path)
	if err != nil {
		return cert, err
	}
	//write to file and close it
	err = pem.Encode(pemCertFile, pemCertBlock)
	if err != nil {
		return cert, err
	}
	pemCertFile.Close()
	fmt.Println("certificate generated and written to", path)
	return cert, nil
}
