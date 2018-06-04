// helper function to create a cert template with a serial number and other required fields
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"
)

func main() {
	//generate private key and write to .pem file
	privateKey, err := CreatePrivateKey()
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

	//create cert and write to .pem file
	err = CreateCert(rootCertTmpl, rootCertTmpl, publicKey, privateKey)
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
		SerialNumber:          serialNumber,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour), // valid for an hour
		BasicConstraintsValid: true,
	}
	return &tmpl, nil
}

//CreatePrivateKey generates a private key and saves it to a .pem file
func CreatePrivateKey() (privKey *rsa.PrivateKey, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	//encode private key
	pemPrivateBlock := &pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	//create new file for private key
	pemPrivateFile, err := os.Create("certs/cert_files/private_key.pem")
	if err != nil {
		return privateKey, err
	}
	//write to file and close it
	err = pem.Encode(pemPrivateFile, pemPrivateBlock)
	if err != nil {
		return privateKey, err
	}
	pemPrivateFile.Close()
	fmt.Println("private key generated and written to certs/cert_files/private_key.pem")
	return privateKey, nil
}

//CreateCert creates a cert and saves it to a .pem file
func CreateCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (err error) {
	//cert *x509.Certificate,
	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return err
	}
	pemCertBlock := &pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	//create new file for private key
	pemCertFile, err := os.Create("certs/cert_files/certificate.pem")
	if err != nil {
		return err
	}
	//write to file and close it
	err = pem.Encode(pemCertFile, pemCertBlock)
	if err != nil {
		return err
	}
	pemCertFile.Close()
	fmt.Println("certificate generated and written to certs/cert_files/certificate.pem")
	return nil
}
