package servercapauth

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/pkg/tls"
	"github.com/trimble-oss/tierceron/pkg/trcnet"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"google.golang.org/grpc"
)

var onceMemo sync.Once

type FeatherAuth struct {
	EncryptPass   string
	EncryptSalt   string
	HandshakePort string
	SecretsPort   string
	HandshakeCode string
}

func ValidateTrcshPathSha(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) (bool, error) {

	certifyMap, err := mod.ReadData(cursoropts.BuildOptions.GetTrcshConfigPath())
	if err != nil {
		return false, err
	}

	if _, ok := certifyMap["trcsha256"]; ok {
		peerExe, err := os.Open(cursoropts.BuildOptions.GetTrcshBinPath())
		if err != nil {
			return false, err
		}
		defer peerExe.Close()

		return true, nil
		// TODO: Check previous 10 versions?  If any match, then
		// return ok....

		// if _, err := io.Copy(h, peerExe); err != nil {
		// 	return false, err
		// }
		// if certifyMap["trcsha256"].(string) == hex.EncodeToString(h.Sum(nil)) {
		// 	return true, nil
		// }
	}
	return false, errors.New("missing certification")
}

func Init(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) (*FeatherAuth, error) {

	certifyMap, err := mod.ReadData(cursoropts.BuildOptions.GetTrcshConfigPath())
	if err != nil {
		return nil, err
	}

	if _, ok := certifyMap["trcsha256"]; ok {
		logger.Println("Registering cap auth.")

		go func() {
			retryCap := 0
			for retryCap < 5 {
				//err := cap.Tap("/home/jrieke/workspace/Github/tierceron/plugins/deploy/target/trcsh", certifyMap["trcsha256"].(string), "azuredeploy", true)
				//err := tap.Tap("/home/jrieke/workspace/Github/tierceron/trcsh/__debug_bin", certifyMap["trcsha256"].(string), "azuredeploy", true)
				tapMap := map[string]string{
					cursoropts.BuildOptions.GetTrcshBinPath(): certifyMap["trcsha256"].(string),
				
				}

				err := tap.Tap(tapMap, "azuredeploy", false)
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

	if !cursoropts.BuildOptions.IsCursor() {
		return nil, nil
	}

	if pluginConfig["env"] == "staging" || pluginConfig["env"] == "prod" {
		// Feathering not supported in staging/prod non messenger at this time.
		featherMap, _ := mod.ReadData(cursoropts.BuildOptions.GetCursorConfigPath())
		if _, ok := featherMap["trcHatSecretsPort"]; ok {
			featherAuth := &FeatherAuth{EncryptPass: "", EncryptSalt: "", HandshakePort: "", SecretsPort: featherMap["trcHatSecretsPort"].(string), HandshakeCode: ""}
			return featherAuth, nil
		}

		logger.Println("Mad hat cap failure port init.")
		return nil, nil
	}

	logger.Println("Feathering check.")
	featherMap, _ := mod.ReadData(cursoropts.BuildOptions.GetCursorConfigPath())
	// TODO: enable error validation when secrets are stored...
	// if err != nil {
	// 	return nil, err
	// }
	if featherMap != nil {
		okMap := true
		for _, key := range []string{
			"trcHatEncryptPass",
			"trcHatEncryptSalt",
			"trcHatHandshakePort",
			"trcHatHandshakeCode",
			"trcHatSecretsPort"} {
			if keyI, ok := featherMap[key]; ok {
				if _, ok := keyI.(string); !ok {
					logger.Printf("Bad %s\n", key)
					okMap = false
					break
				}
			} else {
				logger.Printf("Bad %s\n", key)
				okMap = false
				break
			}
		}

		if okMap {
			logger.Println("Feathering provided.")
			featherAuth := &FeatherAuth{EncryptPass: featherMap["trcHatEncryptPass"].(string), EncryptSalt: featherMap["trcHatEncryptSalt"].(string), HandshakePort: featherMap["trcHatHandshakePort"].(string), SecretsPort: featherMap["trcHatSecretsPort"].(string), HandshakeCode: featherMap["trcHatHandshakeCode"].(string)}
			return featherAuth, nil
		} else {
			logger.Println("Feathering skipped.  Misconfigured.")
		}

	} else {
		logger.Println("Feathering skipped.")
	}

	return nil, nil
}

func Memorize(memorizeFields map[string]interface{}, logger *log.Logger) {
	for key, value := range memorizeFields {
		switch key {
		case "trcHatSecretsPort":
			// Insecure things can be remembered here...
			logger.Println("EyeRemember: " + key)
			tap.TapEyeRemember(key, value.(string))
		case "vaddress", "caddress", "configrole":
			cap.TapFeather(key, value.(string))
			fallthrough
		case "pubrole", "token", "kubeconfig":
			logger.Println("Memorizing: " + key)
			cap.TapMemorize(key, value.(string))
		default:
			logger.Println("Skipping key: " + key)
		}
	}
}

// Things to make available to trusted agent.
func Start(featherAuth *FeatherAuth, env string, logger *log.Logger) error {
	logger.Println("Cap server.")

	creds, credErr := tls.GetServerCredentials(logger)
	if credErr != nil {
		logger.Printf("Couldn't server creds: %v\n", creds)
		return credErr
	}

	logger.Println("Cap creds.")

	localip, err := trcnet.LocalIp()
	if err != nil {
		logger.Printf("Couldn't load ip: %v\n", err)
		return err
	}

	if featherAuth != nil && (len(featherAuth.EncryptPass) > 0 || len(featherAuth.SecretsPort) > 0) {
		cap.TapInitCodeSaltGuard(saltyopts.BuildOptions.GetSaltyGuardian)
	}

	if featherAuth != nil && len(featherAuth.EncryptPass) > 0 {
		logger.Println("Feathering server.")
		go cap.Feather(featherAuth.EncryptPass,
			featherAuth.EncryptSalt,
			fmt.Sprintf("%s:%s", localip, featherAuth.HandshakePort),
			featherAuth.HandshakeCode,
			func(int, string) bool {
				return true
			},
		)
		logger.Println("Feathered server.")
	} else {
		logger.Println("Missing optional feather configuration.  trcsh virtual machine based service deployments will be disabled.")
	}

	if featherAuth != nil && len(featherAuth.SecretsPort) > 0 {
		logger.Println("Tapping server.")
		cap.TapServer(fmt.Sprintf("%s:%s", localip, featherAuth.SecretsPort), grpc.Creds(creds))
		logger.Println("Server tapped.")
	} else {
		logger.Println("Missing optional detailed feather configuration.  trcsh virtual machine based service deployments will be disabled.")
	}

	return nil
}
