package capauth

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var onceMemo sync.Once

type FeatherAuth struct {
	EncryptPass   string
	EncryptSalt   string
	Port          string
	HandshakeCode string
}

var trcshaPath string = "/home/azuredeploy/bin/trcsh"

const (
	ServCert = "/etc/opt/vault/certs/serv_cert.pem"
	ServKey  = "/etc/opt/vault/certs/serv_key.pem"
)

var MashupCertPool *x509.CertPool

func init() {
	rand.Seed(time.Now().UnixNano())
	mashupCertBytes, err := os.ReadFile(ServCert)
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

func ValidatePathSha(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) (bool, error) {

	certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/trcsh/Certify")
	if err != nil {
		return false, err
	}

	if _, ok := certifyMap["trcsha256"]; ok {
		h := sha256.New()

		peerExe, err := os.Open(trcshaPath)
		if err != nil {
			return false, err
		}
		defer peerExe.Close()

		if _, err := io.Copy(h, peerExe); err != nil {
			return false, err
		}

		if certifyMap["trcsha256"].(string) == hex.EncodeToString(h.Sum(nil)) {
			return true, nil
		}
	}
	return false, errors.New("missing certification")
}

func Init(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) (*FeatherAuth, error) {

	certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/trcsh/Certify")
	if err != nil {
		return nil, err
	}

	if _, ok := certifyMap["trcsha256"]; ok {
		logger.Println("Registering cap auth.")
		go func() {
			retryCap := 0
			for retryCap < 5 {
				//err := cap.Tap("/home/jrieke/workspace/Github/tierceron/trcvault/deploy/target/trcsh", certifyMap["trcsha256"].(string), "azuredeploy", true)
				//err := cap.Tap("/home/jrieke/workspace/Github/tierceron/trcsh/__debug_bin", certifyMap["trcsha256"].(string), "azuredeploy", true)
				err := cap.Tap("/home/azuredeploy/bin/trcsh", certifyMap["trcsha256"].(string), "azuredeploy", false)
				if err != nil {
					logger.Println("Cap failure with error: " + err.Error())
					retryCap++
				} else {
					retryCap = 0
				}
			}
			logger.Println("Mad hat cap failure.")
		}()
	}
	if pluginConfig["env"] == "staging" || pluginConfig["env"] == "prod" {
		// Feathering not supported in staging/prod at this time.
		return nil, nil
	}
	featherMap, _ := mod.ReadData("super-secrets/Restricted/TrcshAgent/config")
	// TODO: enable error validation when secrets are stored...
	// if err != nil {
	// 	return nil, err
	// }
	if featherMap != nil {
		if _, ok := featherMap["trcHatEncryptPass"]; ok {
			if _, ok := featherMap["trcHatEncryptSalt"]; ok {
				if _, ok := featherMap["trcHatHandshakePort"]; ok {
					if _, ok := featherMap["trcHatHandshakeCode"]; ok {
						featherAuth := &FeatherAuth{EncryptPass: featherMap["trcHatEncryptPass"].(string), EncryptSalt: featherMap["trcHatEncryptSalt"].(string), Port: featherMap["trcHatHandshakePort"].(string), HandshakeCode: featherMap["trcHatHandshakeCode"].(string)}
						return featherAuth, nil
					}
				}
			}
		}
	}

	return nil, nil
}

func Memorize(memorizeFields map[string]interface{}, logger *log.Logger) {
	for key, value := range memorizeFields {
		switch key {
		case "vaddress", "configrole":
			cap.TapFeather(key, value.(string))
			fallthrough
		case "pubrole", "kubeconfig", "ctoken", "caddress":
			logger.Println("Memorizing: " + key)
			cap.TapMemorize(key, value.(string))
		default:
			logger.Println("Skipping key: " + key)
		}
	}
}

// Things to make available to trusted agent.
func Start(featherAuth *FeatherAuth, logger *log.Logger) error {
	mashupCertBytes, err := os.ReadFile(ServCert)
	if err != nil {
		logger.Printf("Couldn't load cert: %v\n", err)
		return err
	}

	mashupKeyBytes, err := os.ReadFile(ServKey)
	if err != nil {
		logger.Printf("Couldn't load key: %v\n", err)
		return err
	}

	cert, err := tls.X509KeyPair(mashupCertBytes, mashupKeyBytes)
	if err != nil {
		logger.Printf("Couldn't load cert: %v\n", err)
		return err
	}
	creds := credentials.NewServerTLSFromCert(&cert)
	logger.Println("Cap server.")

	if featherAuth != nil {
		logger.Println("Feathering server.")
		go cap.Feather(featherAuth.EncryptPass,
			featherAuth.EncryptSalt,
			featherAuth.Port,
			featherAuth.HandshakeCode,
			func(int, string) bool {
				return true
			},
		)
		logger.Println("Feathered server.")
	}

	// TODO: make port configured and stored in vault.
	logger.Println("Tapping server.")
	cap.TapServer("127.0.0.1:12384", grpc.Creds(creds))
	logger.Println("Server tapped.")

	return nil
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func PenseQuery(encryptPass string, encryptSalt string, handshakeHostPort string, handshakeCode string, penseHostPort string, pense string) (string, error) {
	penseCode := randomString(7 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	_, featherErr := cap.FeatherWriter(encryptPass, encryptSalt, handshakeHostPort, handshakeCode, penseSum)
	if featherErr != nil {
		return "", featherErr
	}

	conn, err := grpc.Dial(penseHostPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
	if err != nil {
		return "", err
	}

	return r.GetPense(), nil
}
