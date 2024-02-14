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
	ServCert           = "/etc/opt/vault/certs/serv_cert.pem"
	ServCertPrefixPath = "/etc/opt/vault/certs/"
	ServCertLocal      = "./serv_cert.pem"
	ServKey            = "/etc/opt/vault/certs/serv_key.pem"
)

var MashupCertPool *x509.CertPool

func ReadServerCert(certName string) ([]byte, error) {
	var err error
	if len(certName) == 0 {
		if _, err = os.Stat(ServCert); err == nil {
			return os.ReadFile(ServCert)
		}
	} else if _, err = os.Stat(ServCertPrefixPath + certName); err == nil { //To support &certName=??
		return os.ReadFile(ServCertPrefixPath + certName)
	} else {
		if utils.IsWindows() {
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

func init() {
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

func LocalIp(env string) (string, error) {

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if strings.HasPrefix(iface.Name, "eth0") {
			addrs, err := iface.Addrs()
			if err != nil {
				fmt.Println("Error getting addresses for", iface.Name, ":", err)
				continue
			}

			for _, address := range addrs {
				// Check if address belongs to eth0
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						return ipnet.IP.String(), nil
					}
				}
			}
		}
	}
	return "", err
}

func LocalAddr(env string) (string, error) {
	localIP, err := LocalIp(env)
	if err != nil {
		return "", err
	}
	addrs, hostErr := net.LookupAddr(localIP)
	if hostErr != nil {
		return "", hostErr
	}
	localHost := ""
	if len(addrs) > 0 {
		if len(addrs) > 20 {
			return "", errors.New("unsupported hosts")
		}
		for _, addr := range addrs {
			localHost = strings.TrimRight(addr, ".")
			if validErr := ValidateVhost(localHost, ""); validErr != nil {
				localHost = ""
				continue
			} else {
				break
			}
		}
	} else {
		return "", errors.New("invalid host")
	}

	return localHost, nil
}

func GetTransportCredentials() (credentials.TransportCredentials, error) {

	mashupKeyBytes, err := ReadServerCert("")
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
