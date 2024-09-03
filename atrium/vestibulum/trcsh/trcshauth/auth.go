package trcshauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

const configDir = "/.tierceron/config.yml"
const envContextPrefix = "envContext: "

func GetSetEnvAddrContext(env string, envContext string, addrPort string) (string, string, string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}

	//This will use env by default, if blank it will use context. If context is defined, it will replace context.
	if env == "" {
		file, err := os.ReadFile(dirname + configDir)
		if err != nil {
			fmt.Printf("Could not read the context file due to this %s error \n", err)
			return "", "", "", err
		}
		fileContent := string(file)
		if fileContent == "" {
			return "", "", "", errors.New("could not read the context file")
		}
		if !strings.Contains(fileContent, envContextPrefix) && envContext != "" {
			var output string
			if !strings.HasSuffix(fileContent, "\n") {
				output = fileContent + "\n" + envContextPrefix + envContext + "\n"
			} else {
				output = fileContent + envContextPrefix + envContext + "\n"
			}

			if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
				return "", "", "", err
			}
			fmt.Println("Context flag has been written out.")
			env = envContext
		} else {
			re := regexp.MustCompile(`[-]?\d[\d,]*[\.]?[\d{2}]*`)
			result := re.FindAllString(fileContent[:strings.Index(fileContent, "\n")], -1)
			if len(result) == 1 {
				addrPort = result[0]
			} else {
				return "", "", "", errors.New("couldn't find port")
			}
			currentEnvContext := strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
			if envContext != "" {
				output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
				if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
					return "", "", "", err
				}
				fmt.Println("Context flag has been written out.")
				env = envContext
			} else if env == "" {
				env = currentEnvContext
				envContext = currentEnvContext
			}
		}
	} else {
		envContext = env
		fmt.Println("Context flag will be ignored as env is defined.")
	}
	return env, envContext, addrPort, nil
}

func retryingPenseFeatherQuery(featherCtx *cap.FeatherContext, agentConfigs *capauth.AgentConfigs, pense string) (*string, error) {
	retry := 0
	for retry < 5 {
		result, err := agentConfigs.PenseFeatherQuery(featherCtx, pense)

		if err != nil || result == nil || *result == "...." {
			time.Sleep(time.Second)
			retry = retry + 1
		} else {
			return result, err
		}
	}
	return nil, errors.New("unavailable secrets")
}

func TrcshVAddress(featherCtx *cap.FeatherContext, agentConfigs *capauth.AgentConfigs, trcshDriverConfig *capauth.TrcshDriverConfig) (*string, error) {
	var err error
	var vaultAddress *string

	if featherCtx != nil {
		vaultAddress, err = retryingPenseFeatherQuery(featherCtx, agentConfigs, "caddress")
	} else {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Auth phase 0")
		vaultAddress, err = capauth.PenseQuery(trcshDriverConfig, "caddress")
	}
	return vaultAddress, err
}

// Helper function for obtaining auth components.
func TrcshAuth(featherCtx *cap.FeatherContext, agentConfigs *capauth.AgentConfigs, trcshDriverConfig *capauth.TrcshDriverConfig) (*capauth.TrcShConfig, error) {
	trcshConfig := &capauth.TrcShConfig{}
	var err error

	if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "staging" || trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "prod" || len(trcshDriverConfig.DriverConfig.TrcShellRaw) > 0 {
		dir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("No homedir for current user")
			os.Exit(1)
		}
		fileBytes, err := os.ReadFile(dir + "/.kube/config")
		if err != nil {
			fmt.Println("No local kube config found...")
			os.Exit(1)
		}
		kc := base64.StdEncoding.EncodeToString(fileBytes)
		trcshConfig.KubeConfig = &kc

		if len(trcshDriverConfig.DriverConfig.TrcShellRaw) > 0 {
			return trcshConfig, nil
		}
	} else {
		if featherCtx == nil {
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Auth phase 1")
			trcshConfig.KubeConfig, err = capauth.PenseQuery(trcshDriverConfig, "kubeconfig")
		}
	}

	if err != nil {
		return trcshConfig, err
	}
	if trcshConfig.KubeConfig != nil {
		memprotectopts.MemProtect(nil, trcshConfig.KubeConfig)
	}

	if featherCtx != nil {
		trcshConfig.VaultAddress, err = retryingPenseFeatherQuery(featherCtx, agentConfigs, "caddress")
	} else {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Auth phase 2")
		trcshConfig.VaultAddress, err = capauth.PenseQuery(trcshDriverConfig, "caddress")
	}
	if err != nil {
		return trcshConfig, err
	}

	memprotectopts.MemProtect(nil, trcshConfig.VaultAddress)

	if err != nil {
		var addrPort string
		var env, envContext string

		fmt.Println(err)
		//Env should come from command line - not context here. but addr port is needed.
		trcshConfig.Env, trcshConfig.EnvContext, addrPort, err = GetSetEnvAddrContext(env, envContext, addrPort)
		if err != nil {
			fmt.Println(err)
			return trcshConfig, err
		}
		vAddr := fmt.Sprintf("https://127.0.0.1:%s", addrPort)
		trcshConfig.VaultAddress = &vAddr

		trcshDriverConfig.DriverConfig.CoreConfig.Env = env
		trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis = env
	}

	trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress = *trcshConfig.VaultAddress
	memprotectopts.MemProtect(nil, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress)

	if featherCtx != nil {
		trcshConfig.ConfigRole, err = retryingPenseFeatherQuery(featherCtx, agentConfigs, "configrole")
	} else {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Auth phase 3")
		trcshConfig.ConfigRole, err = capauth.PenseQuery(trcshDriverConfig, "configrole")
	}
	if err != nil {
		return trcshConfig, err
	}

	memprotectopts.MemProtect(nil, trcshConfig.ConfigRole)

	if featherCtx == nil {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Auth phase 4")
		trcshConfig.PubRole, err = capauth.PenseQuery(trcshDriverConfig, "pubrole")
		if err != nil {
			return trcshConfig, err
		}
		memprotectopts.MemProtect(nil, trcshConfig.PubRole)
	}

	if featherCtx == nil {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Auth phase 6")
		trcshConfig.Token, err = capauth.PenseQuery(trcshDriverConfig, "token")
		if err != nil {
			return trcshConfig, err
		}
		memprotectopts.MemProtect(nil, trcshConfig.Token)
	}
	if err != nil {
		return trcshConfig, err
	}

	trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Auth complete.")

	return trcshConfig, err
}

func ValidateTrcshPathSha(mod *kv.Modifier, pluginConfig map[string]interface{}, logger *log.Logger) (bool, error) {
	certifyPath := "super-secrets/Index/TrcVault/trcplugin/trcsh/Certify"
	var pluginName string
	if plugin, ok := pluginConfig["plugin"].(string); ok {
		certifyPath = "super-secrets/Index/TrcVault/trcplugin/" + plugin + "/Certify"
		pluginName = plugin
	}
	certifyMap, err := mod.ReadData(certifyPath)
	if err != nil {
		fmt.Printf("Error reading data from vault: %s\n", err)
		logger.Printf("Error reading data from vault: %s\n", err)
		return false, err
	}

	ex, err := os.Executable()
	if err != nil {
		fmt.Printf("Unable to access executable: %s\n", err)
		logger.Printf("Unable to access executable: %s\n", err)
		return false, err
	}
	exPath := filepath.Dir(ex)
	trcshaPath := exPath + string(os.PathSeparator)
	trcshaPath = trcshaPath + pluginName

	if _, ok := certifyMap["trcsha256"]; ok {
		peerExe, err := os.Open(trcshaPath)
		if err != nil {
			fmt.Printf("Unable to open executable: %s\n", err)
			logger.Printf("Unable to open executable: %s\n", err)
			return false, err
		}

		defer peerExe.Close()

		h := sha256.New()
		if _, err := io.Copy(h, peerExe); err != nil {
			fmt.Printf("Unable to copy file: %s\n", err)
			logger.Printf("Unable to copy file: %s\n", err)
			return false, err
		}
		sha := hex.EncodeToString(h.Sum(nil))
		if certifyMap["trcsha256"].(string) == sha {
			logger.Println("Validated drone")
			return true, nil
		} else {
			logger.Printf("Error obtaining authorization components from drone: %s\n", errors.New("missing certification"))
			return false, errors.New("missing certification from drone")
		}
	}
	logger.Printf("Missing certification from Vault")
	return false, errors.New("missing certification from Vault")
}
