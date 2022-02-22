package utils

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	sys "tierceron/vaulthelper/system"

	"tierceron/vaulthelper/kv"

	"gopkg.in/yaml.v2"
)

type cert struct {
	VaultHost string `yaml:"vaultHost"`
	ApproleID string `yaml:"approleID"`
	SecretID  string `yaml:"secretID"`
}

var prodRegions = []string{"west", "east", "ca"}

func GetSupportedProdRegions() []string {
	return prodRegions
}

func (c *cert) getCert(logger *log.Logger) (*cert, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		logger.Printf("User home directory #%v ", err)
	}

	yamlFile, err := ioutil.ReadFile(userHome + "/.tierceron/config.yml")
	if err != nil {
		logger.Printf("yamlFile.Get err #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return nil, err
	}

	return c, err
}

// AutoAuth attempts to authenticate a user.
func AutoAuth(config *DriverConfig,
	secretIDPtr *string,
	appRoleIDPtr *string,
	tokenPtr *string,
	tokenNamePtr *string,
	envPtr *string,
	addrPtr *string,
	ping bool) error {
	// Declare local variables
	var override bool
	var exists bool
	var c cert
	var v *sys.Vault

	if tokenPtr != nil && *tokenPtr != "" && addrPtr != nil && *addrPtr != "" {
		// For token based auth, auto auth not
		return errors.New("Missing auth.")
	}

	// Get current user's home directory
	userHome, err := os.UserHomeDir()
	if err != nil {
		config.Log.Printf("User home directory #%v ", err)
	}

	// New values available for the cert file
	if secretIDPtr != nil && *secretIDPtr != "" && appRoleIDPtr != nil && *appRoleIDPtr != "" {
		override = true
	}

	// If cert file exists obtain secretID and appRoleID
	if strings.Index(*envPtr, "staging") == 0 || strings.Index(*envPtr, "prod") == 0 {
		override = false
		exists = true
		appRoleIDPtr = nil
		secretIDPtr = nil
	} else {
		if _, err := os.Stat(userHome + "/.tierceron/config.yml"); !os.IsNotExist(err) {
			exists = true
			_, certErr := c.getCert(config.Log)
			if certErr != nil {
				return certErr
			}

			if addrPtr == nil || *addrPtr == "" {
				*addrPtr = c.VaultHost
			}

			if *tokenPtr == "" {
				if !override {
					LogInfo("Grabbing config IDs from cert file.", config.Log)
					if c.SecretID != "" && secretIDPtr != nil {
						*secretIDPtr = c.SecretID
					}
					if c.ApproleID != "" && appRoleIDPtr != nil {
						*appRoleIDPtr = c.ApproleID
					}
				}
			}
		}
	}

	// Overriding or first time access: request IDs and create cert file
	if override || !exists {
		var vaultHost string
		var secretID string
		var approleID string
		var dump []byte

		if override {
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			// Enter ID tokens
			LogInfo("No cert file found, please enter config IDs", config.Log)
			if addrPtr != nil && *addrPtr != "" {
				LogInfo("vaultHost: "+*addrPtr, config.Log)
				vaultHost = *addrPtr
			} else {
				LogInfo("vaultHost: ", config.Log)
				scanner.Scan()
				vaultHost = scanner.Text()
			}

			if *tokenPtr == "" {
				if secretIDPtr != nil && *secretIDPtr != "" {
					LogInfo("secretID: "+*secretIDPtr, config.Log)
					secretID = *secretIDPtr
				} else if secretIDPtr != nil {
					LogInfo("secretID: ", config.Log)
					scanner.Scan()
					secretID = scanner.Text()
					*secretIDPtr = secretID
				}

				if appRoleIDPtr != nil && *appRoleIDPtr != "" {
					LogInfo("approleID: "+*appRoleIDPtr, config.Log)
					approleID = *appRoleIDPtr
				} else if appRoleIDPtr != nil {
					LogInfo("approleID: ", config.Log)
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
		v, err = sys.NewVault(config.Insecure, *addrPtr, *envPtr, false, ping, false, config.Log)
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
		} else if override && !exists {
			LogInfo("No cert file exists, continuing without saving config IDs", config.Log)
		} else {
			LogInfo(fmt.Sprintf("Creating new cert file in %s", userHome+"/.tierceron/config.yml \n"), config.Log)
			certConfigData := "vaultHost: " + vaultHost + "\n"
			if appRoleIDPtr != nil && secretIDPtr != nil {
				certConfigData = certConfigData + "approleID: " + *appRoleIDPtr + "\nsecretID: " + *secretIDPtr
			}

			dump = []byte(certConfigData)
		}

		// Do not save IDs if overriding and no cert file exists
		if !override || exists {

			// Create hidden folder
			if _, err := os.Stat(userHome + "/.tierceron"); os.IsNotExist(err) {
				err = os.MkdirAll(userHome+"/.tierceron", 0700)
				if err != nil {
					return err
				}
			}

			// Create cert file
			writeErr := ioutil.WriteFile(userHome+"/.tierceron/config.yml", dump, 0600)
			if writeErr != nil {
				LogInfo(fmt.Sprintf("Unable to write file: %v\n", writeErr), config.Log)
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
		v, err = sys.NewVault(config.Insecure, *addrPtr, *envPtr, false, ping, false, config.Log)
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
		env, _, _, envErr := kv.PreCheckEnvironment(*envPtr)
		if envErr != nil {
			LogErrorMessage(fmt.Sprintf("Environment format error: %v\n", envErr), config.Log, false)
			os.Exit(-1)
		}

		switch env {
		case "dev":
			*tokenNamePtr = "config_token_dev"
		case "QA":
			*tokenNamePtr = "config_token_QA"
		case "RQA":
			*tokenNamePtr = "config_token_RQA"
		case "itdev":
			*tokenNamePtr = "config_token_itdev"
		case "performance":
			*tokenNamePtr = "config_token_performance"
		case "servicepack":
			*tokenNamePtr = "config_token_servicepack"
		case "local":
			*tokenNamePtr = "config_token_local"
		default:
			*tokenNamePtr = "Invalid environment"
		}
		//check that none are empty
		if *secretIDPtr == "" {
			return LogAndSafeExit(config, "Missing required secretID", 1)
		} else if *appRoleIDPtr == "" {
			return LogAndSafeExit(config, "Missing required appRoleID", 1)
		} else if *tokenNamePtr == "" {
			return LogAndSafeExit(config, "Missing required tokenName", 1)
		} else if *tokenNamePtr == "Invalid environment" {
			return LogAndSafeExit(config, "Invalid environment:"+*envPtr, 1)
		}
		//check that token matches environment
		tokenParts := strings.Split(*tokenNamePtr, "_")
		tokenEnv := tokenParts[len(tokenParts)-1]
		if env != tokenEnv {
			return LogAndSafeExit(config, "Token doesn't match environment", 1)
		}
	}

	if len(*tokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			return errors.New("Need both public and secret app role to retrieve token from vault")
		}

		master, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
		if err != nil {
			return err
		}

		mod, err := kv.NewModifier(config.Insecure, master, *addrPtr, *envPtr, nil, config.Log)
		if err != nil {
			return err
		}
		mod.Env = "bamboo"

		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		if err != nil {
			return err
		}
	}
	return nil
}
