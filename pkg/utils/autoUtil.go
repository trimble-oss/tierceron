package utils

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"

	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"gopkg.in/yaml.v2"
)

type cert struct {
	VaultHost string `yaml:"vaultHost"`
	ApproleID string `yaml:"approleID"`
	SecretID  string `yaml:"secretID"`
	EnvCtx    string `yaml:"envCtx"`
}

var prodRegions = []string{"west", "east", "ca"}

func GetSupportedProdRegions() []string {
	return prodRegions
}

func (c *cert) getConfig(logger *log.Logger, file string) (*cert, error) {
	userHome, err := userHome(logger)
	if err != nil {
		return nil, err
	}

	yamlFile, err := os.ReadFile(userHome + "/.tierceron/" + file)
	if err != nil {
		logger.Printf("yamlFile.Get err #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return nil, err
	}

	return c, err
}

func userHome(logger *log.Logger) (string, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		logger.Printf("User home directory #%v ", err)
		return "", err
	}
	return userHome, err
}

const configDir = "/.tierceron/config.yml"
const envContextPrefix = "envContext: "

func GetSetEnvContext(env string, envContext string) (string, string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	//This will use env by default, if blank it will use context. If context is defined, it will replace context.
	if env == "" {
		if _, errNotExist := os.Stat(dirname + configDir); errNotExist == nil {
			file, err := os.ReadFile(dirname + configDir)
			if err != nil {
				fmt.Printf("Could not read the context file due to this %s error \n", err)
				return "", "", err
			}
			fileContent := string(file)
			if fileContent == "" {
				return "", "", errors.New("could not read the context file")
			}
			if !strings.Contains(fileContent, envContextPrefix) && envContext != "" {
				var output string
				if !strings.HasSuffix(fileContent, "\n") {
					output = fileContent + "\n" + envContextPrefix + envContext + "\n"
				} else {
					output = fileContent + envContextPrefix + envContext + "\n"
				}

				if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
					return "", "", err
				}
				fmt.Println("Context flag has been written out.")
				env = envContext
			} else {
				currentEnvContext := "dev"
				if strings.Index(fileContent, envContextPrefix) > 0 {
					currentEnvContext = strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
				}
				if envContext != "" {
					output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
					if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
						return "", "", err
					}
					fmt.Println("Context flag has been written out.")
					env = envContext
				} else if env == "" {
					env = currentEnvContext
					envContext = currentEnvContext
				}
			}
		} else {
			env = "dev"
			envContext = "dev"
		}
	} else {
		envContext = env
		fmt.Println("Context flag will be ignored as env is defined.")
	}
	return env, envContext, nil
}

// AutoAuth attempts to authenticate a user.
func AutoAuth(driverConfig *config.DriverConfig,
	wantedTokenNamePtr *string,
	tokenProvidedPtr **string,
	envPtr *string,
	envCtxPtr *string,
	roleEntityPtr *string,
	ping bool) error {
	// Declare local variables
	var override bool
	var exists bool
	var v *sys.Vault

	var appRoleSecret *[]string
	addrPtr := driverConfig.CoreConfig.TokenCache.VaultAddressPtr
	if addrPtr == nil {
		addrPtr = new(string)
	}
	appRoleSecret = driverConfig.CoreConfig.TokenCache.GetRoleStr(roleEntityPtr)
	if appRoleSecret == nil {
		appRoleSecret = new([]string)
		(*appRoleSecret) = append((*appRoleSecret), "")
		(*appRoleSecret) = append((*appRoleSecret), "")
	}
	var tokenPtr *string
	if RefLength(wantedTokenNamePtr) > 0 {
		tokenPtr = driverConfig.CoreConfig.TokenCache.GetToken(*wantedTokenNamePtr)
		if !driverConfig.CoreConfig.IsShell && tokenProvidedPtr != nil {
			driverConfig.CoreConfig.CurrentTokenNamePtr = wantedTokenNamePtr
		}
	}
	if tokenPtr == nil && RefLength(*tokenProvidedPtr) > 0 {
		tokenPtr = *tokenProvidedPtr
		// Make thebig assumption here.
		driverConfig.CoreConfig.TokenCache.AddToken(*wantedTokenNamePtr, tokenPtr)
	}
	if RefLength(tokenPtr) != 0 &&
		!RefEquals(addrPtr, "") &&
		!RefEquals(roleEntityPtr, "deployauth") &&
		!RefEquals(roleEntityPtr, "hivekernel") &&
		(driverConfig.CoreConfig.CurrentTokenNamePtr == nil && wantedTokenNamePtr != nil ||
			// Accept provided token if:
			// 1. current nil, wanted not nil.
			// 2. both nil.
			// 3. both not nil and equal
			RefRefEquals(wantedTokenNamePtr, driverConfig.CoreConfig.CurrentTokenNamePtr)) {
		// For token based auth, auto auth not
		*tokenProvidedPtr = tokenPtr
		return nil
	}
	var err error

	// New values available for the cert file
	if len((*appRoleSecret)[0]) > 0 && len((*appRoleSecret)[1]) > 0 {
		override = true
	}

	// If cert file exists obtain secretID and appRoleID
	var cEnvCtx string
	if RefLength(roleEntityPtr) == 0 {
		roleEntityPtr = new(string)
	}
	if len((*appRoleSecret)[0]) == 0 && len((*appRoleSecret)[1]) == 0 {
		if driverConfig.IsShellSubProcess {
			return errors.New("required azure deploy approle and secret are missing")
		}
		var errAuth error
		readAuthParts := !override && (RefLength(tokenPtr) == 0 ||
			!RefEquals(driverConfig.CoreConfig.CurrentTokenNamePtr, *wantedTokenNamePtr))

		exists, cEnvCtx, errAuth = ReadAuthParts(driverConfig, readAuthParts)
		if errAuth != nil {
			return errAuth
		} else {
			appRoleSecretFromCert := driverConfig.CoreConfig.TokenCache.GetRoleStr(driverConfig.CoreConfig.CurrentRoleEntityPtr)
			if RefLength(addrPtr) == 0 {
				addrPtr = driverConfig.CoreConfig.TokenCache.VaultAddressPtr
			}
			if appRoleSecretFromCert != nil {
				appRoleSecret = appRoleSecretFromCert
			}
		}
	}

	// If no token provided but context is provided, prefer the context over env.
	if tokenPtr == nil &&
		envCtxPtr != nil &&
		(envPtr == nil || len(*envPtr) == 0) {
		envPtr = envCtxPtr
	} else {
		if (envPtr == nil || len(*envPtr) == 0) && cEnvCtx != "" {
			envPtr = &cEnvCtx
		}
	}

	// Overriding or first time access: request IDs and create cert file
	if override || !exists {
		var vaultHost string
		var secretID string
		var approleID string
		var dump []byte

		if override || RefEquals(roleEntityPtr, "deployauth") || RefEquals(roleEntityPtr, "hivekernel") {
			// Nothing...
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			// Enter ID tokens
			if !driverConfig.CoreConfig.IsProd() {
				LogInfo(driverConfig.CoreConfig, "No cert file found, please enter config IDs")
			} else {
				LogInfo(driverConfig.CoreConfig, "Please enter config IDs")
			}
			if addrPtr != nil && *addrPtr != "" {
				LogInfo(driverConfig.CoreConfig, "vaultHost: "+*addrPtr)
				vaultHost = *addrPtr
			} else {
				LogInfo(driverConfig.CoreConfig, "vaultHost: ")
				scanner.Scan()
				vaultHost = scanner.Text()
			}

			if RefLength(tokenPtr) == 0 {
				if len((*appRoleSecret)[1]) > 0 {
					secretID = (*appRoleSecret)[1]
				} else {
					LogInfo(driverConfig.CoreConfig, "secretID: ")
					scanner.Scan()
					secretID = scanner.Text()
					(*appRoleSecret)[1] = secretID
				}

				if len((*appRoleSecret)[0]) > 0 {
					secretID = (*appRoleSecret)[1]
				} else {
					LogInfo(driverConfig.CoreConfig, "approleID: ")
					scanner.Scan()
					approleID = scanner.Text()
					(*appRoleSecret)[0] = secretID
				}
			}

			if strings.HasPrefix(vaultHost, "http://") {
				vaultHost = strings.Replace(vaultHost, "http://", "https://", 1)
			} else if !strings.HasPrefix(vaultHost, "https://") {
				vaultHost = "https://" + vaultHost
			}
			*addrPtr = vaultHost

			// Checks that the scanner is working
			if err := scanner.Err(); err != nil {
				return err
			}
		}
		if driverConfig.CoreConfig.IsShell {
			driverConfig.CoreConfig.Log.Printf("Auth connecting to vault @ %s and env: %s\n", *addrPtr, *envPtr)
		} else {
			fmt.Printf("Auth connecting to vault @ %s\n", *addrPtr)
		}
		v, err = sys.NewVault(driverConfig.CoreConfig.Insecure, addrPtr, *envPtr, false, ping, false, driverConfig.CoreConfig.Log)
		if v != nil {
			defer v.Close()
		} else {
			if ping {
				return nil
			}
		}
		if err != nil {
			return err
		}

		// Get dump
		if override && exists {
			certConfigData := "vaultHost: " + *addrPtr + "\n"
			if len((*appRoleSecret)[0]) > 0 && len((*appRoleSecret)[1]) > 0 {
				certConfigData = certConfigData + "approleID: " + (*appRoleSecret)[0] + "\nsecretID: " + (*appRoleSecret)[1]
			}

			dump = []byte(certConfigData)
		} else if (override && !exists) || RefEquals(roleEntityPtr, "deployauth") || RefEquals(roleEntityPtr, "hivekernel") {
			if !driverConfig.CoreConfig.IsShell {
				LogInfo(driverConfig.CoreConfig, "No approle file exists, continuing without saving config IDs")
			}
		} else {
			// Get current user's home directory
			userHome, err := userHome(driverConfig.CoreConfig.Log)
			if err != nil {
				return err
			}
			driverConfig.CoreConfig.Log.Printf("User home directory %v ", userHome)

			LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Creating new cert file in %s", userHome+"/.tierceron/config.yml \n"))
			certConfigData := "vaultHost: " + vaultHost + "\n"
			if len((*appRoleSecret)[0]) > 0 && len((*appRoleSecret)[1]) > 0 {
				certConfigData = certConfigData + "approleID: " + (*appRoleSecret)[0] + "\nsecretID: " + (*appRoleSecret)[1]
			}

			if envCtxPtr != nil {
				certConfigData = certConfigData + "\nenvCtx: " + *envCtxPtr
			}
			dump = []byte(certConfigData)
		}

		// Do not save IDs if overriding and no approle file exists
		if !driverConfig.CoreConfig.IsProd() && (!override || exists) && !RefEquals(roleEntityPtr, "deployauth") && !RefEquals(roleEntityPtr, "hivekernel") {
			// Get current user's home directory
			userHome, err := userHome(driverConfig.CoreConfig.Log)
			if err != nil {
				return err
			}
			driverConfig.CoreConfig.Log.Printf("User home directory %v ", userHome)

			// Create hidden folder
			if _, err := os.Stat(userHome + "/.tierceron"); os.IsNotExist(err) {
				err = os.MkdirAll(userHome+"/.tierceron", 0700)
				if err != nil {
					return err
				}
			}

			// Create cert file
			writeErr := os.WriteFile(userHome+"/.tierceron/config.yml", dump, 0600)
			if writeErr != nil {
				LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Unable to write file: %v\n", writeErr))
			}
		}

		// Set config IDs
		if !override {
			if len(approleID) > 0 && len(secretID) > 0 {
				role := []string{approleID, secretID}
				driverConfig.CoreConfig.TokenCache.AddRole("bamboo", &role)
			}
		}
	} else {
		fmt.Printf("No override auth connecting to vault @ %s\n", *addrPtr)
		v, err = sys.NewVault(driverConfig.CoreConfig.Insecure, addrPtr, *envPtr, false, ping, false, driverConfig.CoreConfig.Log)

		if v != nil {
			defer v.Close()
		} else {
			if ping {
				return nil
			}
		}
		if err != nil {
			return err
		}
	}

	if len((*appRoleSecret)[0]) == 0 || len((*appRoleSecret)[1]) == 0 {
		// Vaultinit and vaultx may take this path.
		return nil
	}

	//if using appRole
	if len((*appRoleSecret)[0]) > 0 || len((*appRoleSecret)[1]) > 0 && *wantedTokenNamePtr != "" {
		env, _, _, envErr := helperkv.PreCheckEnvironment(*envPtr)
		if envErr != nil {
			LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Environment format error: %v\n", envErr), false)
			return envErr
		}

		tokenNamePrefix := "config"
		// The next two lines break trcplgtool codebundledeploy...
		// if driverConfig.CoreConfig.IsShell && RefLength(tokenNamePtr) > 0 && *tokenNamePtr != "pluginany" {
		// 	goto skipswitch
		// }
		switch *roleEntityPtr {
		case "configpub.yml":
			tokenNamePrefix = "vault_pub"
		case "configdeploy.yml":
			tokenNamePrefix = "vault_token_deploy"
			goto skipswitch
		case "deployauth":
			tokenNamePrefix = "vault_token_azuredeploy"
			goto skipswitch
		case "hivekernel":
			tokenNamePrefix = "trcsh_agent"
			*wantedTokenNamePtr = tokenNamePrefix + "_" + GetEnvBasis(env)
			goto skipswitch
		}
		switch GetEnvBasis(env) {
		case "dev":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_dev"
		case "QA":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_QA"
		case "RQA":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_RQA"
		case "itdev":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_itdev"
		case "performance":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_performance"
		case "staging":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_staging"
		case "prod":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_prod"
		case "servicepack":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_servicepack"
		case "auto":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_auto"
		case "local":
			*wantedTokenNamePtr = tokenNamePrefix + "_token_local"
		default:
			*wantedTokenNamePtr = "Invalid environment"
		}
	skipswitch:
		//check that none are empty
		if len((*appRoleSecret)[1]) == 0 {
			return LogAndSafeExit(driverConfig.CoreConfig, "Missing required secretID", 1)
		} else if len((*appRoleSecret)[0]) == 0 {
			return LogAndSafeExit(driverConfig.CoreConfig, "Missing required appRoleID", 1)
		} else if *wantedTokenNamePtr == "" {
			return LogAndSafeExit(driverConfig.CoreConfig, "Missing required tokenName", 1)
		} else if *wantedTokenNamePtr == "Invalid environment" {
			return LogAndSafeExit(driverConfig.CoreConfig, "Invalid environment:"+*envPtr, 1)
		}
		//check that token matches environment
		tokenParts := strings.Split(*wantedTokenNamePtr, "_")
		tokenEnv := tokenParts[len(tokenParts)-1]
		if GetEnvBasis(env) != tokenEnv {
			return LogAndSafeExit(driverConfig.CoreConfig, "Token doesn't match environment", 1)
		}
	}

	if len(*wantedTokenNamePtr) > 0 && RefLength(driverConfig.CoreConfig.CurrentTokenNamePtr) == 0 {
		if len((*appRoleSecret)[0]) == 0 || len((*appRoleSecret)[1]) == 0 {
			return errors.New("need both public and secret app role to retrieve token from vault")
		}

		if len((*appRoleSecret)[0]) != 36 || len((*appRoleSecret)[1]) != 36 {
			return fmt.Errorf("unexpected approle len = %d and secret len = %d --> expecting 36", len((*appRoleSecret)[0]), len((*appRoleSecret)[1]))
		}

		roleToken, err := v.AppRoleLogin((*appRoleSecret)[0], (*appRoleSecret)[1])
		if err != nil {
			return err
		}

		mod, err := helperkv.NewModifier(driverConfig.CoreConfig.Insecure, roleToken, addrPtr, *envPtr, nil, false, driverConfig.CoreConfig.Log)
		if mod != nil {
			defer mod.Release()
		}
		if err != nil {
			return err
		}
		mod.EnvBasis = "bamboo"
		mod.Env = "bamboo"
		switch *roleEntityPtr {
		case "configpub.yml":
			mod.EnvBasis = "pub"
			mod.Env = "pub"
		case "configdeploy.yml":
			mod.EnvBasis = "deploy"
			mod.Env = "deploy"
		case "deployauth":
			mod.EnvBasis = "azuredeploy"
			mod.Env = "azuredeploy"
		case "hivekernel":
			mod.EnvBasis = "hivekernel"
			mod.Env = "hivekernel"
		}
		LogInfo(driverConfig.CoreConfig, "Detected and utilizing role: "+mod.Env)
		token, err := mod.ReadValue("super-secrets/tokens", *wantedTokenNamePtr)
		if err != nil {
			if strings.Contains(err.Error(), "permission denied") {
				mod.Env = "sugarcane"
				sugarToken, sugarErr := mod.ReadValue("super-secrets/tokens", *wantedTokenNamePtr+"_protected")
				if sugarErr != nil {
					return err
				}
				token = sugarToken
			} else {
				return err
			}
		}
		tokenPtr = &token
		driverConfig.CoreConfig.CurrentTokenNamePtr = wantedTokenNamePtr
		driverConfig.CoreConfig.TokenCache.AddToken(*wantedTokenNamePtr, tokenPtr)
		*tokenProvidedPtr = tokenPtr
	}
	LogInfo(driverConfig.CoreConfig, "Auth credentials obtained.")
	return nil
}

func ReadAuthParts(driverConfig *config.DriverConfig,
	readAuthParts bool) (bool, string, error) {
	exists := false
	var c cert
	if !driverConfig.CoreConfig.IsProd() {
		// Get current user's home directory
		userHome, err := userHome(driverConfig.CoreConfig.Log)
		roleFile := "config.yml"
		if err != nil {
			return false, "", err
		}
		driverConfig.CoreConfig.Log.Printf("User home directory %v ", userHome)
		if _, err := os.Stat(userHome + "/.tierceron/" + roleFile); !os.IsNotExist(err) {
			exists = true
			_, configErr := c.getConfig(driverConfig.CoreConfig.Log, roleFile)
			if configErr != nil {
				return false, "", configErr
			}
			if RefLength(driverConfig.CoreConfig.TokenCache.VaultAddressPtr) == 0 {
				driverConfig.CoreConfig.TokenCache.SetVaultAddress(&c.VaultHost)
			}

			if readAuthParts {
				LogInfo(driverConfig.CoreConfig, "Obtaining auth credentials.")
				if c.ApproleID != "" && c.SecretID != "" {
					role := []string{c.ApproleID, c.SecretID}
					bambooRole := "bamboo"
					driverConfig.CoreConfig.CurrentRoleEntityPtr = &bambooRole
					driverConfig.CoreConfig.TokenCache.AddRole(bambooRole, &role)
				}
			}
		} else {
			driverConfig.CoreConfig.Log.Printf("Invalid home directory %v ", err)
		}
	}
	return exists, c.EnvCtx, nil
}
