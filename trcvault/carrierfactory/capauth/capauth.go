package capauth

import (
	"crypto/tls"
	"log"
	"sync"

	"github.com/trimble-oss/tierceron/trcsh/trcshauth"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var onceMemo sync.Once

func Init(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) error {

	certifyMap, err := mod.ReadData("super-secrets/Index/TrcVault/trcplugin/trcsh/Certify")
	if err != nil {
		return err
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
				}
				retryCap++
			}
			logger.Println("Mad hat cap failure with error: " + err.Error())
		}()
		logger.Println("Memorizing")
		go MemorizeAndStart(pluginConfig, logger)
	}
	return nil
}

// Things to make available to trusted agent.
func MemorizeAndStart(memorizeFields map[string]interface{}, logger *log.Logger) error {
	for key, value := range memorizeFields {
		switch key {
		case "vaddress", "pubrole", "configrole", "kubeconfig":
			logger.Println("Memorizing: " + key)
			cap.TapMemorize(key, value.(string))
		default:
			logger.Println("Skipping key: " + key)
		}
	}
	mashupCertBytes, err := trcshauth.MashupCert.ReadFile("tls/mashup.crt")
	if err != nil {
		logger.Printf("Couldn't load cert: %v\n", err)
		return err
	}

	mashupKeyBytes, err := trcshauth.MashupKey.ReadFile("tls/mashup.key")
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
	logger.Println("Tapping server.")

	// TODO: make port configured and stored in vault.
	cap.TapServer("127.0.0.1:12384", grpc.Creds(creds))
	logger.Println("Server tapped.")

	return nil
}
