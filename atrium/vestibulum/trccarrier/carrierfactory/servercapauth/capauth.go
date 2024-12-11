package servercapauth

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron-hat/cap/tap"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/saltyopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/tls"
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

// ValidateTrcshPathSha - if at least one plugin is properly certified, return true.
func ValidateTrcshPathSha(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) (bool, error) {
	logger.Printf("ValidateTrcshPathSha start\n")

	trustsMap := cursoropts.BuildOptions.GetTrusts()
	logger.Printf("Validating %d trusts\n", len(trustsMap))

	for _, trustData := range trustsMap {
		logger.Printf("Validating %s\n", trustData[0])

		certifyMap, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", trustData[0]))
		if err != nil {
			logger.Printf("Validating Certification failure for %s %s\n", trustData[0], err)
			continue
		}

		if _, ok := certifyMap["trcsha256"]; ok {
			peerExe, err := os.Open(trustData[1])
			if err != nil {
				logger.Printf("ValidateTrcshPathSha complete with file open error %s\n", err.Error())
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
		} else {
			logger.Printf("Missing certification trcsha256 for %s\n", trustData[0])
			continue
		}
	}

	logger.Printf("ValidateTrcshPathSha completing with failure\n")
	return false, errors.New("missing certification")
}

func Init(mod *kv.Modifier, pluginConfig map[string]interface{}, wantsFeathering bool, logger *log.Logger) (*FeatherAuth, error) {

	trustsMap := cursoropts.BuildOptions.GetTrusts()
	tapMap := map[string]string{}
	tapGroup := "azuredeploy"
	for _, trustData := range trustsMap {
		certifyMap, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", trustData[0]))
		if err != nil {
			logger.Printf("Certification failure on expected plugin: %s\n", trustData[0])
			continue
		}
		if _, ok := certifyMap["trcsha256"].(string); ok {
			logger.Println("Registering cap auth.")
			tapGroup = trustData[2]
			tapMap[trustData[1]] = certifyMap["trcsha256"].(string)
		}
	}

	go func() {
		retryCap := 0
		for retryCap < 5 {
			//err := cap.Tap("/home/jrieke/workspace/Github/tierceron/plugins/deploy/target/trcsh", certifyMap["trcsha256"].(string), "azuredeploy", true)
			//err := tap.Tap("/home/jrieke/workspace/Github/tierceron/trcsh/__debug_bin", certifyMap["trcsha256"].(string), "azuredeploy", true)

			err := tap.Tap(cursoropts.BuildOptions.GetCapPath(), tapMap, tapGroup, false)
			if err != nil {
				logger.Println("Cap failure with error: " + err.Error())
				retryCap++
			} else {
				retryCap = 0
			}
		}
		logger.Println("Mad hat cap failure.")
	}()

	if !wantsFeathering {
		// Feathering not supported in staging/prod non messenger at this time.
		featherMap, _ := mod.ReadData(cursoropts.BuildOptions.GetCursorConfigPath())
		if _, ok := featherMap["trcHatSecretsPort"].(string); ok {
			featherAuth := &FeatherAuth{EncryptPass: "", EncryptSalt: "", HandshakePort: "", SecretsPort: featherMap["trcHatSecretsPort"].(string), HandshakeCode: ""}
			return featherAuth, nil
		} else {
			logger.Println("Invalid format non string trcHatSecretsPort")
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
			logger.Println("Feathering skipped.  Not available.")
			if _, ok := featherMap["trcHatSecretsPort"].(string); ok {
				featherAuth := &FeatherAuth{EncryptPass: "", EncryptSalt: "", HandshakePort: "", SecretsPort: featherMap["trcHatSecretsPort"].(string), HandshakeCode: ""}
				return featherAuth, nil
			} else {
				logger.Println("Invalid format non string trcHatSecretsPort")
			}

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
			valuestring := value.(string)
			tap.TapEyeRemember(key, &valuestring)
		case "trcHatWantsFeathering":
			// Insecure things can be remembered here...
			logger.Println("EyeRemember: " + key)
			valuestring := value.(string)
			tap.TapEyeRemember(key, &valuestring)
		case "vaddress", "caddress":
			valuestring := value.(string)
			cap.TapFeather(key, &valuestring)
			logger.Println("Memorizing: " + key)
			cap.TapMemorize(key, &valuestring)
		case "tokenptr", "configroleptr":
			key, _ = strings.CutSuffix(key, "ptr")
			cap.TapFeather(key, value.(*string))
			fallthrough
		case "ctokenptr", "kubeconfigptr", "pubroleptr":
			key, _ = strings.CutSuffix(key, "ptr")
			logger.Println("Memorizing: " + key)
			cap.TapMemorize(key, value.(*string))
		default:
			logger.Println("Skipping key: " + key)
		}
	}
}

// Things to make available to trusted agent.
func Start(featherAuth *FeatherAuth, env string, logger *log.Logger) error {
	logger.Println("Cap server.")

	creds, credErr := tls.GetServerCredentials(false, logger)
	if credErr != nil {
		logger.Printf("Couldn't server creds: %v\n", creds)
		return credErr
	}

	logger.Println("Cap creds.")

	netIpAddr := capauth.LoopBackAddr()

	if featherAuth != nil && (len(featherAuth.EncryptPass) > 0 || len(featherAuth.SecretsPort) > 0) {
		cap.TapInitCodeSaltGuard(saltyopts.BuildOptions.GetSaltyGuardian)
	}

	if featherAuth != nil && len(featherAuth.EncryptPass) > 0 {
		logger.Println("Feathering server.")
		var err error
		netIpAddr, err = capauth.TrcNetAddr()
		if err != nil {
			logger.Printf("Couldn't load ip: %v\n", err)
			return err
		}

		go cap.Feather(featherAuth.EncryptPass,
			featherAuth.EncryptSalt,
			fmt.Sprintf("%s:%s", netIpAddr, featherAuth.HandshakePort),
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
		cap.TapServer(fmt.Sprintf("%s:%s", netIpAddr, featherAuth.SecretsPort), grpc.Creds(creds))
		logger.Println("Server tapped.")
	} else {
		logger.Println("Missing optional detailed feather configuration.  trcsh virtual machine based service deployments will be disabled.")
	}

	return nil
}
