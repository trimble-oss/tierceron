package vaultinitbase

import (
	"flag"
	"fmt"
	"log"
	"os"

	"Vault.Whoville/utils"
	eUtils "Vault.Whoville/utils"
	"Vault.Whoville/vaulthelper/kv"
	sys "Vault.Whoville/vaulthelper/system"
	il "Vault.Whoville/vaultinit/initlib"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func CommonMain(envPtr *string, addrPtrIn *string) {
	devPtr := flag.Bool("dev", false, "Vault server running in dev mode (does not need to be unsealed)")
	newPtr := flag.Bool("new", false, "New vault being initialized. Creates engines and requests first-time initialization")
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	if addrPtrIn != nil && *addrPtrIn != "" {
		addrPtr = addrPtrIn
	}
	seedPtr := flag.String("seeds", "vault_seeds", "Directory that contains vault seeds")
	tokenPtr := flag.String("token", "", "Vault access token, only use if in dev mode or reseeding")
	shardPtr := flag.String("shard", "", "Key shard used to unseal a vault that has been initialized but restarted")

	namespaceVariable := flag.String("namespace", "", "name of the namespace")

	logFilePtr := flag.String("log", "./var/log/vaultinit.log", "Output path for log files")
	servicePtr := flag.String("service", "", "Seeding vault with a single service")
	prodPtr := flag.Bool("prod", false, "Prod only seeds vault with staging environment")
	uploadCertPtr := flag.Bool("certs", false, "Upload certs if provided")
	rotateTokens := flag.Bool("rotateTokens", false, "rotate tokens")
	tokenExpiration := flag.Bool("tokenExpiration", false, "Look up Token expiration dates")
	pingPtr := flag.Bool("ping", false, "Ping vault.")

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}
	flag.Parse()

	// Prints usage if no flags are specified
	if flag.NFlag() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	namespaceTokenConfigs := "vault_namespaces/" + *namespaceVariable + "/token_files"
	namespaceRoleConfigs := "vault_namespaces/" + *namespaceVariable + "/role_files"
	namespacePolicyConfigs := "vault_namespaces/" + *namespaceVariable + "/policy_files"

	if !*rotateTokens && !*tokenExpiration && !*pingPtr {
		if _, err := os.Stat(*seedPtr); os.IsNotExist(err) {
			fmt.Println("Missing required seed folder: " + *seedPtr)
			os.Exit(1)
		}
	}

	if *prodPtr {
		if *envPtr != "staging" && *envPtr != "prod" {
			flag.Usage()
			os.Exit(1)
		}
	} else {
		if *envPtr == "staging" || *envPtr == "prod" {
			flag.Usage()
			os.Exit(1)
		}
	}
	if !*pingPtr && !*newPtr && *tokenPtr == "" {
		utils.CheckWarning("Missing auth tokens", true)
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("./var/log/"); os.IsNotExist(err) && *logFilePtr == "./var/log/vaultinit.log" {
		*logFilePtr = "./vaultinit.log"
	}

	// Initialize logging
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	utils.CheckError(err, true)
	logger := log.New(f, "[INIT]", log.LstdFlags)
	logger.Println("==========Beginning Vault Initialization==========")

	if addrPtr == nil || *addrPtr == "" {
		eUtils.AutoAuth(nil, nil, tokenPtr, nil, envPtr, addrPtr, *pingPtr)
	}

	// Create a new vault system connection
	v, err := sys.NewVault(*addrPtr, *envPtr, *newPtr, *pingPtr)
	if *pingPtr {
		if err != nil {
			fmt.Printf("Ping failure: %v\n", err)
		}
		os.Exit(0)
	}

	utils.LogErrorObject(err, logger, true)

	// Trying to use local, prompt for username/password
	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		*envPtr, err = utils.LoginToLocal()
		utils.LogErrorObject(err, logger, true)
		logger.Printf("Login successful, using local envronment: %s\n", *envPtr)
	}

	if *devPtr || !*newPtr { // Dev server, initialization taken care of, get root token
		v.SetToken(*tokenPtr)
	} else { // Unseal and grab keys/root token
		keyToken, err := v.InitVault(1, 1)
		utils.LogErrorObject(err, logger, true)
		v.SetToken(keyToken.Token)
		v.SetShards(keyToken.Keys)
		//check error returned by unseal
		_, _, _, err = v.Unseal()
		utils.LogErrorObject(err, logger, true)
	}
	logger.Printf("Succesfully connected to vault at %s\n", *addrPtr)

	if !*newPtr && *namespaceVariable != "" && *namespaceVariable != "vault" {
		//
		// Special path for generating custom scoped tokens.  This path requires
		// a root token usually.
		//

		// Upload new cidr roles.
		il.UploadTokenCidrRoles(namespaceRoleConfigs, v, logger)
		// Upload policies from the given policy directory
		il.UploadPolicies(namespacePolicyConfigs, v, false, logger)
		// Upload tokens from the given token directory
		tokens := il.UploadTokens(namespaceTokenConfigs, v, logger)
		if len(tokens) > 0 {
			logger.Println(*namespaceVariable + " tokens successfully created.")
		} else {
			logger.Println(*namespaceVariable + " tokens failed to create.")
		}
		f.Close()
		os.Exit(0)
	}

	if !*newPtr && (*rotateTokens || *tokenExpiration) {
		getOrRevokeError := v.GetOrRevokeTokensInScope(namespaceTokenConfigs, *tokenExpiration, logger)
		if getOrRevokeError != nil {
			fmt.Println("Token revocation or access failure.  Cannot continue.")
			os.Exit(-1)
		}
		if !*tokenExpiration {
			tokens := il.UploadTokens(namespaceTokenConfigs, v, logger)
			if !*prodPtr {
				tokenMap := map[string]interface{}{}
				for _, token := range tokens {
					tokenMap[token.Name] = token.Value
				}

				mod, err := kv.NewModifier(v.GetToken(), *addrPtr, "nonprod", nil) // Connect to vault
				utils.LogErrorObject(err, logger, false)

				mod.Env = "bamboo"
				warn, err := mod.Write("super-secrets/tokens", tokenMap)
				utils.LogErrorObject(err, logger, false)
				utils.LogWarningsObject(warn, logger, false)
			}
		}
		os.Exit(0)
	}

	// Try to unseal if an old vault and unseal key given
	if !*newPtr && len(*shardPtr) > 0 {
		v.AddShard(*shardPtr)
		prog, t, success, err := v.Unseal()
		utils.LogErrorObject(err, logger, true)
		if !success {
			logger.Printf("Vault unseal progress: %d/%d key shards\n", prog, t)
			logger.Println("============End Initialization Attempt============")
		} else {
			logger.Println("Vault successfully unsealed")
		}
	}

	if *newPtr {
		// Create secret engines
		il.CreateEngines(v, logger)
		// Upload policies from the given policy directory
		il.UploadPolicies(namespacePolicyConfigs, v, false, logger)
		// Upload tokens from the given token directory
		tokens := il.UploadTokens(namespacePolicyConfigs, v, logger)
		if !*prodPtr {
			tokenMap := map[string]interface{}{}
			for _, token := range tokens {
				tokenMap[token.Name] = token.Value
			}

			mod, err := kv.NewModifier(v.GetToken(), *addrPtr, "nonprod", nil) // Connect to vault
			utils.LogErrorObject(err, logger, false)

			mod.Env = "bamboo"
			warn, err := mod.Write("super-secrets/tokens", tokenMap)
			utils.LogErrorObject(err, logger, false)
			utils.LogWarningsObject(warn, logger, false)
		}
	}

	// Seed the vault with given seed directory
	il.SeedVault(*seedPtr, *addrPtr, v.GetToken(), *envPtr, logger, *servicePtr, *uploadCertPtr)

	logger.SetPrefix("[INIT]")
	logger.Println("=============End Vault Initialization=============")
	logger.Println()

	// Uncomment this when deployed to avoid a hanging root token
	// v.RevokeSelf()
	f.Close()
}
