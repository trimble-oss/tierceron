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
	secretIDPtr *string, // Optional if token provided.
	appRoleIDPtr *string, // Optional if token provided.
	wantedTokenNamePtr *string,
	tokenProvidedPtr **string,
	envPtr *string,
	addrPtr *string,
	envCtxPtr *string,
	appRoleConfigPtr *string,
	ping bool) error {
	// Declare local variables
	var override bool
	var exists bool
	var c cert
	var v *sys.Vault

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
		!RefEquals(appRoleConfigPtr, "deployauth") &&
		!RefEquals(appRoleConfigPtr, "hivekernel") &&
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
	// Get current user's home directory
	userHome, err := userHome(driverConfig.CoreConfig.Log)
	if err != nil {
		return err
	}

	// New values available for the cert file
	if secretIDPtr != nil && len(*secretIDPtr) > 0 && appRoleIDPtr != nil && len(*appRoleIDPtr) > 0 {
		override = true
	}
	isProd := strings.Contains(*envPtr, "staging") || strings.Contains(*envPtr, "prod")

	// If cert file exists obtain secretID and appRoleID
	driverConfig.CoreConfig.Log.Printf("User home directory %v ", userHome)
	if RefLength(appRoleConfigPtr) == 0 {
		appRoleConfigPtr = new(string)
		*appRoleConfigPtr = "config.yml"
	}
	if appRoleIDPtr == nil || len(*appRoleIDPtr) == 0 || secretIDPtr == nil || len(*secretIDPtr) == 0 {
		if driverConfig.IsShellSubProcess {
			return errors.New("required azure deploy approle and secret are missing")
		}
		if !isProd {
			if _, err := os.Stat(userHome + "/.tierceron/" + *appRoleConfigPtr); !os.IsNotExist(err) {
				exists = true
				_, configErr := c.getConfig(driverConfig.CoreConfig.Log, *appRoleConfigPtr)
				if configErr != nil {
					return configErr
				}

				if addrPtr == nil || *addrPtr == "" {
					*addrPtr = c.VaultHost
				}

				if RefLength(tokenPtr) == 0 || *wantedTokenNamePtr != *driverConfig.CoreConfig.CurrentTokenNamePtr {
					if !override {
						LogInfo(driverConfig.CoreConfig, "Obtaining auth credentials.")
						if c.SecretID != "" && secretIDPtr != nil {
							*secretIDPtr = c.SecretID
						}
						if c.ApproleID != "" && appRoleIDPtr != nil {
							*appRoleIDPtr = c.ApproleID
						}
					}
				}
			} else {
				driverConfig.CoreConfig.Log.Printf("Invalid home directory %v ", err)
			}
		}
	}

	// If no token provided but context is provided, prefer the context over env.
	if tokenPtr == nil &&
		envCtxPtr != nil &&
		(envPtr == nil || len(*envPtr) == 0) {
		envPtr = envCtxPtr
	} else {
		if (envPtr == nil || len(*envPtr) == 0) && c.EnvCtx != "" {
			envPtr = &c.EnvCtx
		}
	}

	// Overriding or first time access: request IDs and create cert file
	if override || !exists {
		var vaultHost string
		var secretID string
		var approleID string
		var dump []byte

		if override || RefEquals(appRoleConfigPtr, "deployauth") || RefEquals(appRoleConfigPtr, "hivekernel") {
			// Nothing...
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			// Enter ID tokens
			if !isProd {
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
				if RefLength(secretIDPtr) != 0 {
					secretID = *secretIDPtr
				} else if secretIDPtr != nil {
					LogInfo(driverConfig.CoreConfig, "secretID: ")
					scanner.Scan()
					secretID = scanner.Text()
					*secretIDPtr = secretID
				}

				if RefLength(appRoleIDPtr) != 0 {
					approleID = *appRoleIDPtr
				} else if appRoleIDPtr != nil {
					LogInfo(driverConfig.CoreConfig, "approleID: ")
					scanner.Scan()
					approleID = scanner.Text()
					*appRoleIDPtr = approleID
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
			if appRoleIDPtr != nil && secretIDPtr != nil {
				certConfigData = certConfigData + "approleID: " + *appRoleIDPtr + "\nsecretID: " + *secretIDPtr
			}

			dump = []byte(certConfigData)
		} else if (override && !exists) || RefEquals(appRoleConfigPtr, "deployauth") || RefEquals(appRoleConfigPtr, "hivekernel") {
			if !driverConfig.CoreConfig.IsShell {
				LogInfo(driverConfig.CoreConfig, "No approle file exists, continuing without saving config IDs")
			}
		} else {
			LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Creating new cert file in %s", userHome+"/.tierceron/config.yml \n"))
			certConfigData := "vaultHost: " + vaultHost + "\n"
			if appRoleIDPtr != nil && secretIDPtr != nil {
				certConfigData = certConfigData + "approleID: " + *appRoleIDPtr + "\nsecretID: " + *secretIDPtr
			}

			if envCtxPtr != nil {
				certConfigData = certConfigData + "\nenvCtx: " + *envCtxPtr
			}
			dump = []byte(certConfigData)
		}

		// Do not save IDs if overriding and no approle file exists
		if !isProd && (!override || exists) && !RefEquals(appRoleConfigPtr, "deployauth") && !RefEquals(appRoleConfigPtr, "hivekernel") {

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
			if secretIDPtr != nil && appRoleIDPtr != nil {
				*secretIDPtr = secretID
				*appRoleIDPtr = approleID
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

	if secretIDPtr == nil || appRoleIDPtr == nil {
		// Vaultinit and vaultx may take this path.
		return nil
	}

	//if using appRole
	if *secretIDPtr != "" || *appRoleIDPtr != "" || *wantedTokenNamePtr != "" {
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
		switch *appRoleConfigPtr {
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
		if *secretIDPtr == "" {
			return LogAndSafeExit(driverConfig.CoreConfig, "Missing required secretID", 1)
		} else if *appRoleIDPtr == "" {
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

	if len(*wantedTokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			return errors.New("need both public and secret app role to retrieve token from vault")
		}

		if len(*appRoleIDPtr) != 36 || len(*secretIDPtr) != 36 {
			return fmt.Errorf("unexpected approle len = %d and secret len = %d --> expecting 36", len(*appRoleIDPtr), len(*secretIDPtr))
		}

		roleToken, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
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
		switch *appRoleConfigPtr {
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
