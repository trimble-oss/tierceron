package capauth

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron/pkg/utils"
	"google.golang.org/grpc/credentials"
)

const (
	ServCert      = "/etc/opt/vault/certs/serv_cert.pem"
	ServCertLocal = "./serv_cert.pem"
	ServKey       = "/etc/opt/vault/certs/serv_key.pem"
)

var MashupCertPool *x509.CertPool

func ReadServerCert() ([]byte, error) {
	if _, err := os.Stat(ServCert); err == nil {
		return os.ReadFile(ServCert)
	} else {
		if utils.IsWindows() {
			return os.ReadFile(ServCertLocal)
		} else {
			return nil, errors.New("file not found")
		}
	}
}

func GetTlsConfig() (*tls.Config, error) {
	// I don't think we're doing this right...?.?
	// Comment out for now...
	rootCertPool := x509.NewCertPool()
	pem, err := ReadServerCert()
	if err != nil {
		return nil, err
	}
	if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
		return nil, errors.New("couldn't append certs to root.")
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

func init() {
	rand.Seed(time.Now().UnixNano())
	mashupCertBytes, err := ReadServerCert()
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

func LocalIp(env string) (string, error) {
	if strings.Contains(env, "staging") || strings.Contains(env, "prod") {
		return "127.0.0.1", nil
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", err
}

func GetTransportCredentials() (credentials.TransportCredentials, error) {

	mashupKeyBytes, err := ReadServerCert()
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
