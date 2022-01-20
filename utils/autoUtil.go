package utils

import (
	"bufio"
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

func (c *cert) getCert() *cert {
	userHome, err := os.UserHomeDir()
	if err != nil {
		log.Printf("User home directory #%v ", err)
	}

	yamlFile, err := ioutil.ReadFile(userHome + "/.tierceron/config.yml")
	if err != nil {
		log.Printf("yamlFile.Get err #%v ", err)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return c
}

// AutoAuth attempts to
func AutoAuth(insecure bool, secretIDPtr *string, appRoleIDPtr *string, tokenPtr *string, tokenNamePtr *string, envPtr *string, addrPtr *string, ping bool) {
	// Declare local variables
	var override bool
	var exists bool
	var c cert
	var v *sys.Vault

	if tokenPtr != nil && *tokenPtr != "" && addrPtr != nil && *addrPtr != "" {
		// For token based auth, auto auth not
		return
	}

	// Get current user's home directory
	userHome, err := os.UserHomeDir()
	if err != nil {
		log.Printf("User home directory #%v ", err)
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
			c.getCert()
			if addrPtr == nil || *addrPtr == "" {
				*addrPtr = c.VaultHost
			}

			if *tokenPtr == "" {
				if !override {
					fmt.Println("Grabbing config IDs from cert file.")
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
			fmt.Println("No cert file found, please enter config IDs")
			if addrPtr != nil && *addrPtr != "" {
				fmt.Println("vaultHost: " + *addrPtr)
				vaultHost = *addrPtr
			} else {
				fmt.Print("vaultHost: ")
				scanner.Scan()
				vaultHost = scanner.Text()
			}

			if *tokenPtr == "" {
				if secretIDPtr != nil && *secretIDPtr != "" {
					fmt.Println("secretID: " + *secretIDPtr)
					secretID = *secretIDPtr
				} else if secretIDPtr != nil {
					fmt.Print("secretID: ")
					scanner.Scan()
					secretID = scanner.Text()
					*secretIDPtr = secretID
				}

				if appRoleIDPtr != nil && *appRoleIDPtr != "" {
					fmt.Println("approleID: " + *appRoleIDPtr)
					approleID = *appRoleIDPtr
				} else if appRoleIDPtr != nil {
					fmt.Print("approleID: ")
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
				log.Fatal(err)
			}
		}
		v, err = sys.NewVault(insecure, *addrPtr, *envPtr, false, ping, false)
		CheckErrorNoStack(err, true)

		// Get dump
		if override && exists {
			certConfigData := "vaultHost: " + *addrPtr + "\n"
			if appRoleIDPtr != nil && secretIDPtr != nil {
				certConfigData = certConfigData + "approleID: " + *appRoleIDPtr + "\nsecretID: " + *secretIDPtr
			}

			dump = []byte(certConfigData)
		} else if override && !exists {
			fmt.Println("No cert file exists, continuing without saving config IDs")
		} else {
			fmt.Printf("Creating new cert file in %s", userHome+"/.tierceron/config.yml \n")
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
					log.Fatal(err)
				}
			}

			// Create cert file
			writeErr := ioutil.WriteFile(userHome+"/.tierceron/config.yml", dump, 0600)
			if writeErr != nil {
				fmt.Printf("Unable to write file: %v\n", writeErr)
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
		v, err = sys.NewVault(insecure, *addrPtr, *envPtr, false, ping, false)
		CheckErrorNoStack(err, true)
	}

	if secretIDPtr == nil || appRoleIDPtr == nil {
		// Vaultinit and vaultx may take this path.
		return
	}

	//if using appRole
	if *secretIDPtr != "" || *appRoleIDPtr != "" || *tokenNamePtr != "" {
		env, _, _, envErr := kv.PreCheckEnvironment(*envPtr)
		if envErr != nil {
			fmt.Printf("Environment format error: %v\n", envErr)
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
			CheckWarning(fmt.Sprintf("Missing required secretID"), true)
		} else if *appRoleIDPtr == "" {
			CheckWarning(fmt.Sprintf("Missing required appRoleID"), true)
		} else if *tokenNamePtr == "" {
			CheckWarning(fmt.Sprintf("Missing required tokenName"), true)
		} else if *tokenNamePtr == "Invalid environment" {
			CheckWarning(fmt.Sprintf("Invalid environment:"+*envPtr), true)
		}
		//check that token matches environment
		tokenParts := strings.Split(*tokenNamePtr, "_")
		tokenEnv := tokenParts[len(tokenParts)-1]
		if env != tokenEnv {
			CheckWarning(fmt.Sprintf("Token doesn't match environment"), true)
		}
	}

	if len(*tokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			CheckError(fmt.Errorf("Need both public and secret app role to retrieve token from vault"), true)
		}

		master, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
		CheckError(err, true)

		mod, err := kv.NewModifier(insecure, master, *addrPtr, *envPtr, nil)
		CheckError(err, true)
		mod.Env = "bamboo"

		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		CheckError(err, true)
	}
}
