package capauth

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"sync"

	"github.com/trimble-oss/tierceron/trcsh/trcshauth"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var onceMemo sync.Once

// CheckNotSudo -- checks if current user is sudoer and exits if they are.
func CheckNotSudo() {
	sudoer, sudoErr := user.LookupGroup("sudo")
	if sudoErr != nil {
		fmt.Println("Trcsh unable to definitively identify sudoers.")
		os.Exit(-1)
	}
	sudoerGid, sudoConvErr := strconv.Atoi(sudoer.Gid)
	if sudoConvErr != nil {
		fmt.Println("Trcsh unable to definitively identify sudoers.  Conversion error.")
		os.Exit(-1)
	}
	groups, groupErr := os.Getgroups()
	if groupErr != nil {
		fmt.Println("Trcsh unable to definitively identify sudoers.  Missing groups.")
		os.Exit(-1)
	}
	for _, groupId := range groups {
		if groupId == sudoerGid {
			fmt.Println("Trcsh cannot be run with user having sudo privileges.")
			os.Exit(-1)
		}
	}

}

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
			logger.Println("Mad hat cap failure.")
		}()
	}
	return nil
}

func Memorize(memorizeFields map[string]interface{}, logger *log.Logger) {
	for key, value := range memorizeFields {
		switch key {
		case "vaddress", "pubrole", "configrole", "kubeconfig":
			logger.Println("Memorizing: " + key)
			cap.TapMemorize(key, value.(string))
		default:
			logger.Println("Skipping key: " + key)
		}
	}
}

// Things to make available to trusted agent.
func Start(logger *log.Logger) error {
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
