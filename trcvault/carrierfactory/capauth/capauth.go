package capauth

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

type FeatherAuth struct {
	EncryptPass   string
	EncryptSalt   string
	Port          string
	HandshakeCode string
}

var trcshaPath string = "/home/azuredeploy/bin/trcsh"

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
