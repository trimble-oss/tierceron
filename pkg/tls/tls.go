package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/utils"
	"google.golang.org/grpc/credentials"
)

const (
	ServCertLocal = "./serv_cert.pem"
)

var (
	ServCert           = "/certs/serv_cert.pem"
	ServCertPrefixPath = "/certs/"
	ServKey            = "/certs/serv_key.pem"
)

var MashupCertPool *x509.CertPool

func InitRoot() {
	ServCert = coreopts.BuildOptions.GetVaultInstallRoot() + ServCert
	ServCertPrefixPath = coreopts.BuildOptions.GetVaultInstallRoot() + ServCertPrefixPath
	ServKey = coreopts.BuildOptions.GetVaultInstallRoot() + ServKey
	initCertificates()
}

func ReadServerCert(certName string, drone ...*bool) ([]byte, error) {
	var err error
	if len(certName) == 0 {
		if utils.IsWindows() || (len(drone) > 0 && *drone[0]) {
			return os.ReadFile(ServCertLocal)
		}
		if _, err = os.Stat(ServCert); err == nil {
			return os.ReadFile(ServCert)
		}
	} else if _, err = os.Stat(ServCertPrefixPath + certName); err == nil { //To support &certName=??
		return os.ReadFile(ServCertPrefixPath + certName)
	} else {
		if utils.IsWindows() || (len(drone) > 0 && *drone[0]) {
			return os.ReadFile(ServCertLocal)
		}
	}
	return nil, err
}

func GetTlsConfig(certName string) (*tls.Config, error) {
	// I don't think we're doing this right...?.?
	// Comment out for now...
	rootCertPool := x509.NewCertPool()
	pem, err := ReadServerCert(certName)
	if err != nil {
		return nil, err
	}
	if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
		return nil, errors.New("couldn't append certs to root")
	}
	// clientCert := make([]tls.Certificate, 0, 1)
	// certs, err := tls.LoadX509KeyPair(ServCert, ServKey)
	// if err != nil {
	// 	return nil, err
	// }
	// clientCert = append(clientCert, certs)
	return &tls.Config{
		RootCAs: rootCertPool,
		//		Certificates: clientCert,
	}, nil
}

func initCertificates() {
	rand.Seed(time.Now().UnixNano())
	mashupCertBytes, err := ReadServerCert("")
	if err != nil {
		fmt.Println("Cert read failure.")
		return
	}

	mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))

	mashupClientCert, parseErr := x509.ParseCertificate(mashupBlock.Bytes)
	if parseErr != nil {
		fmt.Println("Cert parse read failure.")
		return
	}
	MashupCertPool = x509.NewCertPool()
	MashupCertPool.AddCert(mashupClientCert)
}

func GetTransportCredentials(drone ...*bool) (credentials.TransportCredentials, error) {
	mashupKeyBytes, err := ReadServerCert("", drone...)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		ServerName: "",
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{mashupKeyBytes},
			},
		},
		InsecureSkipVerify: false}), nil
}

func GetServerCredentials(logger *log.Logger) (credentials.TransportCredentials, error) {
	mashupCertBytes, err := os.ReadFile(ServCert)
	if err != nil {
		logger.Printf("Couldn't load cert: %v\n", err)
		return nil, err
	}

	mashupKeyBytes, err := os.ReadFile(ServKey)
	if err != nil {
		logger.Printf("Couldn't load key: %v\n", err)
		return nil, err
	}

	cert, err := tls.X509KeyPair(mashupCertBytes, mashupKeyBytes)
	if err != nil {
		logger.Printf("Couldn't load cert: %v\n", err)
		return nil, err
	}
	return credentials.NewServerTLSFromCert(&cert), nil
}
