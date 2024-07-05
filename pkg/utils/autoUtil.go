package utils

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"

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

// AutoAuth attempts to authenticate a user.
func AutoAuth(driverConfig *DriverConfig,
	secretIDPtr *string, // Optional if token provided.
	appRoleIDPtr *string, // Optional if token provided.
	tokenPtr *string, // Optional if appRole and secret provided.
	tokenNamePtr *string, // Required if approle and secret provided.
	envPtr *string,
	addrPtr *string,
	envCtxPtr *string,
	appRoleConfig string,
	ping bool) error {
	// Declare local variables
	var override bool
	var exists bool
	var c cert
	var v *sys.Vault

	if tokenPtr != nil && *tokenPtr != "" && addrPtr != nil && *addrPtr != "" && appRoleConfig != "deployauth" {
		// For token based auth, auto auth not
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
	if len(appRoleConfig) == 0 {
		appRoleConfig = "config.yml"
	}
	if appRoleIDPtr == nil || len(*appRoleIDPtr) == 0 || secretIDPtr == nil || len(*secretIDPtr) == 0 {
		if driverConfig.IsShellSubProcess {
			return errors.New("required azure deploy approle and secret are missing")
		}
		if !isProd {
			if _, err := os.Stat(userHome + "/.tierceron/" + appRoleConfig); !os.IsNotExist(err) {
				exists = true
				_, configErr := c.getConfig(driverConfig.CoreConfig.Log, appRoleConfig)
				if configErr != nil {
					return configErr
				}

				if addrPtr == nil || *addrPtr == "" {
					*addrPtr = c.VaultHost
				}

				if *tokenPtr == "" {
					if !override {
						LogInfo(&driverConfig.CoreConfig, "Obtaining auth credentials.")
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
	if tokenPtr == nil && envCtxPtr != nil {
		envPtr = envCtxPtr
	} else {
		if envPtr == nil && c.EnvCtx != "" {
			envPtr = &c.EnvCtx
		}
	}

	// Overriding or first time access: request IDs and create cert file
	if override || !exists {
		var vaultHost string
		var secretID string
		var approleID string
		var dump []byte

		if override || appRoleConfig == "deployauth" {
			// Nothing...
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			// Enter ID tokens
			if !isProd {
				LogInfo(&driverConfig.CoreConfig, "No cert file found, please enter config IDs")
			} else {
				LogInfo(&driverConfig.CoreConfig, "Please enter config IDs")
			}
			if addrPtr != nil && *addrPtr != "" {
				LogInfo(&driverConfig.CoreConfig, "vaultHost: "+*addrPtr)
				vaultHost = *addrPtr
			} else {
				LogInfo(&driverConfig.CoreConfig, "vaultHost: ")
				scanner.Scan()
				vaultHost = scanner.Text()
			}

			if *tokenPtr == "" {
				if secretIDPtr != nil && *secretIDPtr != "" {
					if !isProd {
						LogInfo(&driverConfig.CoreConfig, "secretID: "+*secretIDPtr)
					}
					secretID = *secretIDPtr
				} else if secretIDPtr != nil {
					LogInfo(&driverConfig.CoreConfig, "secretID: ")
					scanner.Scan()
					secretID = scanner.Text()
					*secretIDPtr = secretID
				}

				if appRoleIDPtr != nil && *appRoleIDPtr != "" {
					if !isProd {
						LogInfo(&driverConfig.CoreConfig, "approleID: "+*appRoleIDPtr)
					}
					approleID = *appRoleIDPtr
				} else if appRoleIDPtr != nil {
					LogInfo(&driverConfig.CoreConfig, "approleID: ")
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
		v, err = sys.NewVault(driverConfig.Insecure, *addrPtr, *envPtr, false, ping, false, driverConfig.CoreConfig.Log)
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
		} else if (override && !exists) || appRoleConfig == "deployauth" {
			if !driverConfig.CoreConfig.IsShell {
				LogInfo(&driverConfig.CoreConfig, "No approle file exists, continuing without saving config IDs")
			}
		} else {
			LogInfo(&driverConfig.CoreConfig, fmt.Sprintf("Creating new cert file in %s", userHome+"/.tierceron/config.yml \n"))
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
		if !isProd && (!override || exists) && appRoleConfig != "deployauth" {

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
				LogInfo(&driverConfig.CoreConfig, fmt.Sprintf("Unable to write file: %v\n", writeErr))
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
		v, err = sys.NewVault(driverConfig.Insecure, *addrPtr, *envPtr, false, ping, false, driverConfig.CoreConfig.Log)

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
	if *secretIDPtr != "" || *appRoleIDPtr != "" || *tokenNamePtr != "" {
		env, _, _, envErr := helperkv.PreCheckEnvironment(*envPtr)
		if envErr != nil {
			LogErrorMessage(&driverConfig.CoreConfig, fmt.Sprintf("Environment format error: %v\n", envErr), false)
			return envErr
		}

		tokenNamePrefix := "config"
		switch appRoleConfig {
		case "configpub.yml":
			tokenNamePrefix = "vault_pub"
		case "configdeploy.yml":
			tokenNamePrefix = "vault_token_deploy"
			goto skipswitch
		case "deployauth":
			tokenNamePrefix = "vault_token_azuredeploy"
			goto skipswitch
		}
		switch GetEnvBasis(env) {
		case "dev":
			*tokenNamePtr = tokenNamePrefix + "_token_dev"
		case "QA":
			*tokenNamePtr = tokenNamePrefix + "_token_QA"
		case "RQA":
			*tokenNamePtr = tokenNamePrefix + "_token_RQA"
		case "itdev":
			*tokenNamePtr = tokenNamePrefix + "_token_itdev"
		case "performance":
			*tokenNamePtr = tokenNamePrefix + "_token_performance"
		case "staging":
			*tokenNamePtr = tokenNamePrefix + "_token_staging"
		case "prod":
			*tokenNamePtr = tokenNamePrefix + "_token_prod"
		case "servicepack":
			*tokenNamePtr = tokenNamePrefix + "_token_servicepack"
		case "auto":
			*tokenNamePtr = tokenNamePrefix + "_token_auto"
		case "local":
			*tokenNamePtr = tokenNamePrefix + "_token_local"
		default:
			*tokenNamePtr = "Invalid environment"
		}
	skipswitch:
		//check that none are empty
		if *secretIDPtr == "" {
			return LogAndSafeExit(&driverConfig.CoreConfig, "Missing required secretID", 1)
		} else if *appRoleIDPtr == "" {
			return LogAndSafeExit(&driverConfig.CoreConfig, "Missing required appRoleID", 1)
		} else if *tokenNamePtr == "" {
			return LogAndSafeExit(&driverConfig.CoreConfig, "Missing required tokenName", 1)
		} else if *tokenNamePtr == "Invalid environment" {
			return LogAndSafeExit(&driverConfig.CoreConfig, "Invalid environment:"+*envPtr, 1)
		}
		//check that token matches environment
		tokenParts := strings.Split(*tokenNamePtr, "_")
		tokenEnv := tokenParts[len(tokenParts)-1]
		if GetEnvBasis(env) != tokenEnv {
			return LogAndSafeExit(&driverConfig.CoreConfig, "Token doesn't match environment", 1)
		}
	}

	if len(*tokenNamePtr) > 0 {
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

		mod, err := helperkv.NewModifier(driverConfig.Insecure, roleToken, *addrPtr, *envPtr, nil, false, driverConfig.CoreConfig.Log)
		if mod != nil {
			defer mod.Release()
		}
		if err != nil {
			return err
		}
		mod.EnvBasis = "bamboo"
		mod.Env = "bamboo"
		switch appRoleConfig {
		case "configpub.yml":
			mod.EnvBasis = "pub"
			mod.Env = "pub"
		case "configdeploy.yml":
			mod.EnvBasis = "deploy"
			mod.Env = "deploy"
		case "deployauth":
			mod.EnvBasis = "azuredeploy"
			mod.Env = "azuredeploy"
		}
		LogInfo(&driverConfig.CoreConfig, "Detected and utilizing role: "+mod.Env)
		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		if err != nil {
			if strings.Contains(err.Error(), "permission denied") {
				mod.Env = "sugarcane"
				sugarToken, sugarErr := mod.ReadValue("super-secrets/tokens", *tokenNamePtr+"_protected")
				if sugarErr != nil {
					return err
				}
				*tokenPtr = sugarToken
			} else {
				return err
			}
		}
	}
	LogInfo(&driverConfig.CoreConfig, "Auth credentials obtained.")
	return nil
}
