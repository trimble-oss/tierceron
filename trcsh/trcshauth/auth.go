package trcshauth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/capauth"
	eUtils "github.com/trimble-oss/tierceron/utils"
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

func retryingPenseFeatherQuery(agentConfigs *capauth.AgentConfigs, pense string) (*string, error) {
	retry := 0
	for retry < 5 {
		result, err := agentConfigs.PenseFeatherQuery(pense)

		if err != nil || result == nil || *result == "...." {
			time.Sleep(time.Second)
			retry = retry + 1
		} else {
			return result, err
		}
	}
	return nil, errors.New("unavailable secrets")
}

// Helper function for obtaining auth components.
func TrcshAuth(agentConfigs *capauth.AgentConfigs, config *eUtils.DriverConfig) (*capauth.TrcShConfig, error) {
	trcshConfig := &capauth.TrcShConfig{}
	var err error

	if config.EnvRaw == "staging" || config.EnvRaw == "prod" || len(config.TrcShellRaw) > 0 {
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

		if len(config.TrcShellRaw) > 0 {
			return trcshConfig, nil
		}
	} else {
		if agentConfigs == nil {
			config.Log.Println("Auth phase 1")
			trcshConfig.KubeConfig, err = capauth.PenseQuery(config, "kubeconfig")
		}
	}

	if err != nil {
		return trcshConfig, err
	}
	if trcshConfig.KubeConfig != nil {
		memprotectopts.MemProtect(nil, trcshConfig.KubeConfig)
	}

	if agentConfigs != nil {
		trcshConfig.VaultAddress, err = retryingPenseFeatherQuery(agentConfigs, "caddress")
	} else {
		config.Log.Println("Auth phase 2")
		trcshConfig.VaultAddress, err = capauth.PenseQuery(config, "caddress")
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
		vAddr := "https://127.0.0.1:" + addrPort
		trcshConfig.VaultAddress = &vAddr

		config.Env = env
		config.EnvRaw = env
	}

	config.VaultAddress = *trcshConfig.VaultAddress
	memprotectopts.MemProtect(nil, &config.VaultAddress)

	if agentConfigs != nil {
		trcshConfig.ConfigRole, err = retryingPenseFeatherQuery(agentConfigs, "configrole")
	} else {
		config.Log.Println("Auth phase 3")
		trcshConfig.ConfigRole, err = capauth.PenseQuery(config, "configrole")
	}
	if err != nil {
		return trcshConfig, err
	}

	memprotectopts.MemProtect(nil, trcshConfig.ConfigRole)

	if agentConfigs == nil {
		config.Log.Println("Auth phase 4")
		trcshConfig.PubRole, err = capauth.PenseQuery(config, "pubrole")
		if err != nil {
			return trcshConfig, err
		}
		memprotectopts.MemProtect(nil, trcshConfig.PubRole)
	}

	if agentConfigs != nil {
		trcshConfig.CToken, err = retryingPenseFeatherQuery(agentConfigs, "ctoken")
	} else {
		config.Log.Println("Auth phase 5")
		trcshConfig.CToken, err = capauth.PenseQuery(config, "ctoken")
		if err != nil {
			return trcshConfig, err
		}
	}
	if err != nil {
		return trcshConfig, err
	}

	memprotectopts.MemProtect(nil, trcshConfig.CToken)

	config.Log.Println("Auth complete.")

	return trcshConfig, err
}
