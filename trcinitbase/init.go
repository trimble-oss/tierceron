package trcinitbase

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	il "github.com/trimble-oss/tierceron/trcinit/initlib"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
	"github.com/trimble-oss/tierceron/webapi/rpc/apinator"
)

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func CommonMain(envPtr *string,
	addrPtrIn *string,
	envCtxPtr *string,
	flagset *flag.FlagSet,
	argLines []string) {

	if flagset == nil {
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = flag.Usage
		flagset.String("env", "dev", "Environment to configure")
	}

	devPtr := flagset.Bool("dev", false, "Vault server running in dev mode (does not need to be unsealed)")
	newPtr := flagset.Bool("new", false, "New vault being initialized. Creates engines and requests first-time initialization")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	if addrPtrIn != nil && *addrPtrIn != "" {
		addrPtr = addrPtrIn
	}
	seedPtr := flagset.String("seeds", coreopts.GetFolderPrefix(nil)+"_seeds", "Directory that contains vault seeds")
	tokenPtr := flagset.String("token", "", "Vault access token, only use if in dev mode or reseeding")
	shardPtr := flagset.String("shard", "", "Key shard used to unseal a vault that has been initialized but restarted")

	namespaceVariable := flagset.String("namespace", "vault", "name of the namespace")

	logFilePtr := flagset.String("log", "./"+coreopts.GetFolderPrefix(nil)+"init.log", "Output path for log files")
	servicePtr := flagset.String("service", "", "Seeding vault with a single service")
	prodPtr := flagset.Bool("prod", false, "Prod only seeds vault with staging environment")
	uploadCertPtr := flagset.Bool("certs", false, "Upload certs if provided")
	rotateTokens := flagset.Bool("rotateTokens", false, "rotate tokens")
	tokenExpiration := flagset.Bool("tokenExpiration", false, "Look up Token expiration dates")
	pingPtr := flagset.Bool("ping", false, "Ping vault.")
	updateRole := flagset.Bool("updateRole", false, "Update security role")
	updatePolicy := flagset.Bool("updatePolicy", false, "Update security policy")
	initNamespace := flagset.Bool("initns", false, "Init namespace (tokens, policy, and role)")
	insecurePtr := flagset.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	keyShardPtr := flagset.String("totalKeys", "5", "Total number of key shards to make")
	unsealShardPtr := flagset.String("unsealKeys", "3", "Number of key shards needed to unseal")
	tokenFileFilterPtr := flagset.String("filter", "", "Filter files for token rotation.")
	roleFileFilterPtr := flagset.String("approle", "", "Filter files for approle rotation.")
	dynamicPathPtr := flagset.String("dynamicPath", "", "Seed a specific directory in vault.")
	nestPtr := flagset.Bool("nest", false, "Seed a specific directory in vault.")

	// indexServiceExtFilterPtr := flag.String("serviceExtFilter", "", "Specifies which nested services (or tables) to filter") //offset or database
	// indexServiceFilterPtr := flag.String("serviceFilter", "", "Specifies which services (or tables) to filter")              // Table names
	// indexNameFilterPtr := flag.String("indexFilter", "", "Specifies which index names to filter")                            // column index, table to filter.
	// indexValueFilterPtr := flag.String("indexValueFilter", "", "Specifies which index values to filter")                     // column index value to filter on.

	allowNonLocal := false

	args := argLines[1:]
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}
	eUtils.CheckInitFlags(flagset)
	flagset.Parse(argLines[1:])
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, tokenPtr)
	}

	// Prints usage if no flags are specified
	if flagset.NFlag() == 0 {
		flagset.Usage()
		os.Exit(1)
	}

	if *namespaceVariable == "" && *newPtr {
		fmt.Println("Namespace (-namespace) required to initialize a new vault.")
		os.Exit(1)
	}

	if *newPtr {
		currentDir, dirErr := os.Getwd()
		if dirErr != nil {
			fmt.Println("Couldn not retrieve current working directory")
			os.Exit(1)
		}

		if _, err := os.Stat(currentDir + "/vault_namespaces/vault/token_files"); err != nil {
			fmt.Println("Could not locate token files required to initialize a new vault.")
			os.Exit(1)
		}

		if _, err := os.Stat(currentDir + "/vault_namespaces/vault/policy_files"); err != nil {
			fmt.Println("Could not locate policy files  required to initialize a new vault.")
			os.Exit(1)
		}
	}

	// Enter ID tokens
	if *insecurePtr {
		if isLocal, lookupErr := helperkv.IsUrlIp(*addrPtr); isLocal && lookupErr == nil {
			// This is fine...
			fmt.Println("Initialize local vault.")
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			// Enter ID tokens
			fmt.Println("Are you sure you want to connect to non local server with self signed cert(Y): ")
			scanner.Scan()
			skipVerify := scanner.Text()
			if skipVerify == "Y" || skipVerify == "y" {
				// Good to go.
				allowNonLocal = true
			} else {
				fmt.Println("This is a remote host and you did not confirm allow non local.  If this is a remote host with a self signed cert, init will fail.")
				*insecurePtr = false
			}
		}
	}

	if *nestPtr {
		var input string

		fmt.Printf("Are you sure you want to seed nested files? [y|n]: ")
		_, err := fmt.Scanln(&input)
		if err != nil {
			os.Exit(1)
		}
		input = strings.ToLower(input)

		if input != "y" && input != "yes" {
			os.Exit(1)
		}
	}

	if len(*eUtils.ServiceFilterPtr) != 0 && len(*eUtils.IndexNameFilterPtr) == 0 && len(*eUtils.RestrictedPtr) != 0 {
		eUtils.IndexNameFilterPtr = eUtils.ServiceFilterPtr
	}

	var indexSlice = make([]string, 0) //Checks for indexed projects
	if len(*eUtils.IndexedPtr) > 0 {
		indexSlice = append(indexSlice, strings.Split(*eUtils.IndexedPtr, ",")...)
	}

	var restrictedSlice = make([]string, 0) //Checks for restricted projects
	if len(*eUtils.RestrictedPtr) > 0 {
		restrictedSlice = append(restrictedSlice, strings.Split(*eUtils.RestrictedPtr, ",")...)
	}

	if len(*eUtils.ProtectedPtr) > 0 {
		restrictedSlice = append(restrictedSlice, strings.Split(*eUtils.ProtectedPtr, ",")...)
	}

	namespaceTokenConfigs := "vault_namespaces" + string(os.PathSeparator) + "token_files"
	namespaceRoleConfigs := "vault_namespaces" + string(os.PathSeparator) + "role_files"
	namespacePolicyConfigs := "vault_namespaces" + string(os.PathSeparator) + "policy_files"
	namespaceAppRolePolicies := "vault_namespaces" + string(os.PathSeparator) + "approle_files"

	if *namespaceVariable != "" {
		namespaceTokenConfigs = "vault_namespaces" + string(os.PathSeparator) + *namespaceVariable + string(os.PathSeparator) + "token_files"
		namespaceRoleConfigs = "vault_namespaces" + string(os.PathSeparator) + *namespaceVariable + string(os.PathSeparator) + "role_files"
		namespacePolicyConfigs = "vault_namespaces" + string(os.PathSeparator) + *namespaceVariable + string(os.PathSeparator) + "policy_files"
		namespaceAppRolePolicies = "vault_namespaces" + string(os.PathSeparator) + *namespaceVariable + string(os.PathSeparator) + "approle_files"
	}

	if *namespaceVariable == "" && !*rotateTokens && !*tokenExpiration && !*updatePolicy && !*updateRole && !*pingPtr {
		if _, err := os.Stat(*seedPtr); os.IsNotExist(err) {
			fmt.Println("Missing required seed folder: " + *seedPtr)
			os.Exit(1)
		}
	}

	if *prodPtr {
		if !strings.HasPrefix(*envPtr, "staging") && !strings.HasPrefix(*envPtr, "prod") {
			fmt.Println("The prod flag can only be used with the staging or prod env.")
			flag.Usage()
			os.Exit(1)
		}
	} else {
		if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") {
			fmt.Println("The prod flag should be used with the staging or prod env.")
			flag.Usage()
			os.Exit(1)
		}
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix(nil)+"init.log" && os.IsNotExist(err) {
		*logFilePtr = "./" + coreopts.GetFolderPrefix(nil) + "init.log"
	}

	// Initialize logging
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if f != nil {
		defer f.Close()
	}
	logger := log.New(f, "[INIT]", log.LstdFlags)
	logger.Println("==========Beginning Vault Initialization==========")
	config := &eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}
	eUtils.CheckError(config, err, true)

	if !*pingPtr && !*newPtr && *tokenPtr == "" {
		eUtils.CheckWarning(config, "Missing auth tokens", true)
	}

	if addrPtr == nil || *addrPtr == "" {
		if *newPtr {
			fmt.Println("Address must be specified using -addr flag")
			os.Exit(1)
		}
		autoErr := eUtils.AutoAuth(&eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger}, nil, nil, tokenPtr, nil, envPtr, addrPtr, envCtxPtr, "", *pingPtr)
		if autoErr != nil {
			fmt.Println("Auth failure: " + autoErr.Error())
			os.Exit(1)
		}
	}

	// Create a new vault system connection
	v, err := sys.NewVaultWithNonlocal(*insecurePtr, *addrPtr, *envPtr, *newPtr, *pingPtr, false, allowNonLocal, logger)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate signed by unknown authority") {
			fmt.Printf("Attempting to connect to insecure vault or vault with self signed certificate.  If you really wish to continue, you may add -insecure as on option.\n")
		} else if strings.Contains(err.Error(), "no such host") {
			fmt.Printf("failed to connect to vault - missing host")
		} else {
			fmt.Println(err.Error())
		}

		os.Exit(0)
	}
	if *pingPtr {
		if err != nil {
			fmt.Printf("Ping failure: %v\n", err)
		}
		os.Exit(0)
	}

	eUtils.LogErrorObject(config, err, true)

	// Trying to use local, prompt for username/password
	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		*envPtr, err = eUtils.LoginToLocal()
		eUtils.LogErrorObject(config, err, true)
		logger.Printf("Login successful, using local envronment: %s\n", *envPtr)
	}

	if *devPtr || !*newPtr { // Dev server, initialization taken care of, get root token
		v.SetToken(*tokenPtr)
	} else { // Unseal and grab keys/root token
		totalKeyShard, err := strconv.ParseUint(*keyShardPtr, 10, 32)
		if err != nil {
			fmt.Println("Unable to parse totalKeyShard into int")
		}
		unsealShardPtr, err := strconv.ParseUint(*unsealShardPtr, 10, 32)
		if err != nil {
			fmt.Println("Unable to parse unsealShardPtr into int")
		}
		keyToken, err := v.InitVault(int(totalKeyShard), int(unsealShardPtr))
		eUtils.LogErrorObject(config, err, true)
		v.SetToken(keyToken.Token)
		v.SetShards(keyToken.Keys)
		//check error returned by unseal
		_, _, _, err = v.Unseal()
		eUtils.LogErrorObject(config, err, true)
	}
	logger.Printf("Succesfully connected to vault at %s\n", *addrPtr)

	if !*newPtr && *namespaceVariable != "" && *namespaceVariable != "vault" && !(*rotateTokens || *updatePolicy || *updateRole || *tokenExpiration) {
		if *initNamespace {
			fmt.Println("Creating tokens, roles, and policies.")
			policyExists, policyErr := il.GetExistsPolicies(config, namespacePolicyConfigs, v)
			if policyErr != nil {
				eUtils.LogErrorObject(config, policyErr, false)
				fmt.Println("Cannot safely determine policy.")
				os.Exit(-1)
			}

			if policyExists {
				fmt.Printf("Policy exists for policy configurations in directory: %s.  Refusing to continue.\n", namespacePolicyConfigs)
				os.Exit(-1)
			}

			roleExists, roleErr := il.GetExistsRoles(config, namespaceRoleConfigs, v)
			if roleErr != nil {
				eUtils.LogErrorObject(config, roleErr, false)
				fmt.Println("Cannot safely determine role.")
			}

			if roleExists {
				fmt.Printf("Role exists for role configurations in directory: %s.  Refusing to continue.\n", namespaceRoleConfigs)
				os.Exit(-1)
			}

			// Special path for generating custom scoped tokens.  This path requires
			// a root token usually.
			//

			// Upload Create/Update new cidr roles.
			fmt.Println("Creating role")
			il.UploadTokenCidrRoles(config, namespaceRoleConfigs, v)
			// Upload Create/Update policies from the given policy directory

			fmt.Println("Creating policy")
			il.UploadPolicies(config, namespacePolicyConfigs, v, false)

			// Upload tokens from the given token directory
			fmt.Println("Creating tokens")
			tokens := il.UploadTokens(config, namespaceTokenConfigs, tokenFileFilterPtr, v)
			if len(tokens) > 0 {
				logger.Println(*namespaceVariable + " tokens successfully created.")
			} else {
				logger.Println(*namespaceVariable + " tokens failed to create.")
			}
			os.Exit(0)
		} else {
			fmt.Println("initns or rotateTokens required with the namespace paramater.")
			os.Exit(0)
		}

	}

	if !*newPtr && (*updatePolicy || *rotateTokens || *tokenExpiration || *updateRole) {
		if *tokenExpiration {
			fmt.Println("Checking token expiration.")
			roleId, lease, err := v.GetRoleID("bamboo")
			eUtils.LogErrorObject(config, err, false)
			fmt.Println("AppRole id: " + roleId + " expiration is set to (zero means never expire): " + lease)
		} else {
			if *rotateTokens {
				if *tokenFileFilterPtr == "" {
					fmt.Println("Rotating tokens.")
				} else {
					fmt.Println("Adding token: " + *tokenFileFilterPtr)
				}
			}
		}

		if (*rotateTokens || *tokenExpiration) && (*roleFileFilterPtr == "" || *tokenFileFilterPtr != "") {
			getOrRevokeError := v.GetOrRevokeTokensInScope(namespaceTokenConfigs, *tokenFileFilterPtr, *tokenExpiration, logger)
			if getOrRevokeError != nil {
				fmt.Println("Token revocation or access failure.  Cannot continue.")
				os.Exit(-1)
			}
		}

		if *updateRole {
			// Upload Create/Update new cidr roles.
			fmt.Println("Updating role")
			errTokenCidr := il.UploadTokenCidrRoles(config, namespaceRoleConfigs, v)
			if errTokenCidr != nil {
				if *roleFileFilterPtr != "" { //If old way didn't work -> try new way.
					*rotateTokens = true
				} else {
					fmt.Println("Role update failed.  Cannot continue.")
					os.Exit(-1)
				}
			} else {
				fmt.Println("Role updated")
			}
		}

		if *updatePolicy {
			// Upload Create/Update policies from the given policy directory
			fmt.Println("Updating policies")
			errTokenPolicy := il.UploadPolicies(config, namespacePolicyConfigs, v, false)
			if errTokenPolicy != nil {
				fmt.Println("Policy update failed.  Cannot continue.")
				os.Exit(-1)
			} else {
				fmt.Println("Policies updated")
			}
		}

		if *rotateTokens && !*tokenExpiration {
			var tokens []*apinator.InitResp_Token
			if *roleFileFilterPtr == "" || *tokenFileFilterPtr != "" {
				// Create new tokens.
				fmt.Println("Creating new tokens")
				tokens = il.UploadTokens(config, namespaceTokenConfigs, tokenFileFilterPtr, v)
			}

			mod, err := helperkv.NewModifier(*insecurePtr, v.GetToken(), *addrPtr, "nonprod", nil, true, logger) // Connect to vault
			if mod != nil {
				defer mod.Release()
			}

			if err != nil {
				fmt.Println("Error creating modifer.")
				eUtils.LogErrorObject(config, err, false)
				os.Exit(-1)
			}

			// Process existing approles for provided namespace
			approleFiles := il.GetApproleFileNames(config, *namespaceVariable)
			if len(approleFiles) == 0 {
				fmt.Println("No approles found for namespace: " + *namespaceVariable)
			} else {
				for _, approleFile := range approleFiles {
					if *roleFileFilterPtr != "" && approleFile != *roleFileFilterPtr {
						continue
					}
					tokenMap := map[string]interface{}{}
					fileYAML, parseErr := il.ParseApproleYaml(approleFile, *namespaceVariable)
					if parseErr != nil {
						fmt.Println("Read parsing approle yaml file, continuing to next file.")
						eUtils.LogErrorObject(config, parseErr, false)
						continue
					}
					if approleName, ok := fileYAML["Approle_Name"].(string); ok {
						mod.RawEnv = approleName
						mod.Env = approleName
					} else {
						fmt.Println("Read parsing approle name from file, continuing to next file.")
						eUtils.LogErrorObject(config, parseErr, false)
						continue
					}

					var tokenPerms map[interface{}]interface{}
					if permMap, okPerms := fileYAML["Token_Permissions"].(map[interface{}]interface{}); okPerms {
						tokenPerms = permMap
					} else {
						fmt.Println("Read parsing approle token permissions from file, continuing to next file.")
						eUtils.LogErrorObject(config, parseErr, false)
						continue
					}

					if len(tokenPerms) == 0 {
						fmt.Println("Completely skipping " + mod.RawEnv + " as there is no token permission for this approle.")
						continue
					}

					if *tokenFileFilterPtr != "" && *tokenFileFilterPtr != "*" {
						// Filter out tokens that
						if found, ok := tokenPerms[*tokenFileFilterPtr].(bool); !ok && !found {
							fmt.Println("Skipping " + mod.RawEnv + " as there is no token permission for this approle.")
							continue
						}
					}

					existingTokens, err := mod.ReadData("super-secrets/tokens")
					if err != nil {
						fmt.Println("Read existing tokens failure.  Cannot continue.")
						eUtils.LogErrorObject(config, err, false)
						os.Exit(-1)
					}

					// We have names of tokens that were referenced in old role.  Ok to delete the role now.
					//
					// Merge token data.
					//
					// Copy existing, but only if they are still supported... otherwise strip them from the approle.
					for key, valueObj := range existingTokens {
						if value, ok := valueObj.(string); ok {
							if found, ok := tokenPerms[key].(bool); ok && found && key != "admin" && key != "webapp" {
								tokenMap[key] = value
							}
						} else if stringer, ok := valueObj.(fmt.GoStringer); ok {
							if found, ok := tokenPerms[key].(bool); ok && found && key != "admin" && key != "webapp" {
								tokenMap[key] = stringer.GoString()
							}
						}
					}

					//Overwrite with new tokens.
					for _, token := range tokens {
						// Everything but webapp give access by app role and secret.
						if found, ok := tokenPerms[token.Name].(bool); ok && found && token.Name != "admin" && token.Name != "webapp" {
							tokenMap[token.Name] = token.Value
						}
					}

					if *roleFileFilterPtr != "" && *tokenFileFilterPtr == "" {
						// approle provided, tokenFileFilter not provided...
						// This will just be an approler
						mod.Env = *namespaceVariable
						mod.RawEnv = *namespaceVariable

						//
						// Wipe existing role.
						// Recreate the role.
						//
						resp, role_cleanup := v.DeleteRole(mod.RawEnv)
						eUtils.LogErrorObject(config, role_cleanup, false)

						if resp.StatusCode == 404 {
							err = v.EnableAppRole()
							eUtils.LogErrorObject(config, err, true)
						}

						err = v.CreateNewRole(mod.RawEnv, &sys.NewRoleOptions{
							TokenTTL:    "10m",
							TokenMaxTTL: "15m",
							Policies:    []string{mod.RawEnv},
						})
						eUtils.LogErrorObject(config, err, true)

						roleID, _, err := v.GetRoleID(mod.RawEnv)
						eUtils.LogErrorObject(config, err, true)

						secretID, err := v.GetSecretID(mod.RawEnv)
						eUtils.LogErrorObject(config, err, true)

						fmt.Printf("Rotated role id and secret id for %s.\n", mod.RawEnv)
						fmt.Printf("Role ID: %s\n", roleID)
						fmt.Printf("Secret ID: %s\n", secretID)
					} else {
						fmt.Printf("Existing role token count: %d.\n", len(existingTokens))
						fmt.Printf("Adding token to role: %s.\n", mod.RawEnv)
						fmt.Printf("Post add role token count: %d.\n", len(tokenMap))
					}

					// Just update tokens in approle.
					// This could be adding to an existing approle or re-adding to rotated role...
					if len(tokenMap) > 0 {
						warn, err := mod.Write("super-secrets/tokens", tokenMap, config.Log)
						eUtils.LogErrorObject(config, err, true)
						eUtils.LogWarningsObject(config, warn, true)
						fmt.Println("Approle tokens refreshed/updated.")
					}
				}
			}
		}
		os.Exit(0)
	}

	// Try to unseal if an old vault and unseal key given
	if !*newPtr && len(*shardPtr) > 0 {
		v.AddShard(*shardPtr)
		prog, t, success, err := v.Unseal()
		eUtils.LogErrorObject(config, err, true)
		if !success {
			logger.Printf("Vault unseal progress: %d/%d key shards\n", prog, t)
			logger.Println("============End Initialization Attempt============")
		} else {
			logger.Println("Vault successfully unsealed")
		}
	}

	//TODO: Figure out raft storage initialization for -new flag
	if *newPtr {
		mod, err := helperkv.NewModifier(*insecurePtr, v.GetToken(), *addrPtr, "nonprod", nil, true, logger) // Connect to vault
		if mod != nil {
			defer mod.Release()
		}
		eUtils.LogErrorObject(config, err, true)

		mod.Env = "bamboo"

		if mod.Exists("values/metadata") || mod.Exists("templates/metadata") || mod.Exists("super-secrets/metadata") {
			fmt.Println("Vault has been initialized already...")
			os.Exit(1)
		}

		policyExists, err := il.GetExistsPolicies(config, namespacePolicyConfigs, v)
		if policyExists || err != nil {
			fmt.Printf("Vault may be initialized already - Policies exists.\n")
			os.Exit(1)
		}

		// Create secret engines
		il.CreateEngines(config, v)
		// Upload policies from the given policy directory
		il.UploadPolicies(config, namespacePolicyConfigs, v, false)
		// Upload tokens from the given token directory
		tokens := il.UploadTokens(config, namespaceTokenConfigs, tokenFileFilterPtr, v)
		if !*prodPtr {
			tokenMap := map[string]interface{}{}
			for _, token := range tokens {
				tokenMap[token.Name] = token.Value
			}

			err = v.EnableAppRole()
			eUtils.LogErrorObject(config, err, true)

			err = v.CreateNewRole("bamboo", &sys.NewRoleOptions{
				TokenTTL:    "10m",
				TokenMaxTTL: "15m",
				Policies:    []string{"bamboo"},
			})
			eUtils.LogErrorObject(config, err, true)

			roleID, _, err := v.GetRoleID("bamboo")
			eUtils.LogErrorObject(config, err, true)

			secretID, err := v.GetSecretID("bamboo")
			eUtils.LogErrorObject(config, err, true)

			fmt.Printf("Created new role id and secret id for bamboo.\n")
			fmt.Printf("Role ID: %s\n", roleID)
			fmt.Printf("Secret ID: %s\n", secretID)

			files, err := os.ReadDir(namespaceAppRolePolicies)
			appRolePolicies := []string{}
			isPolicy := true
			if err == nil {
				for _, file := range files {
					filename := file.Name()
					ext := filepath.Ext(filename)
					filename = filename[0 : len(filename)-len(ext)]
					isPolicy = true
					fileYAML, parseErr := il.ParseApproleYaml(filename, *namespaceVariable)
					if parseErr != nil {
						isPolicy = false
						fmt.Println("Unable to parse approle yaml file, continuing to next file.")
						eUtils.LogErrorObject(config, parseErr, false)
						continue
					}
					_, okPerms := fileYAML["Token_Permissions"].(map[interface{}]interface{})
					if !okPerms {
						isPolicy = false
						fmt.Println("Read incorrect approle token permissions from file, continuing to next file.")
						eUtils.LogErrorObject(config, parseErr, false)
						continue
					}
					if isPolicy {
						appRolePolicies = append(appRolePolicies, filename)
					}
				}
			}

			//
			// Wipe existing protected app role.
			// Recreate the protected app role.
			//
			for _, appRolePolicy := range appRolePolicies {

				if strings.Contains(appRolePolicy, "bamboo") {
					continue
				}

				resp, role_cleanup := v.DeleteRole(appRolePolicy)
				eUtils.LogErrorObject(config, role_cleanup, false)

				if resp.StatusCode == 404 {
					err = v.EnableAppRole()
					eUtils.LogErrorObject(config, err, true)
				}

				err = v.CreateNewRole(appRolePolicy, &sys.NewRoleOptions{
					TokenTTL:    "10m",
					TokenMaxTTL: "15m",
					Policies:    []string{appRolePolicy},
				})
				eUtils.LogErrorObject(config, err, true)

				appRoleID, _, err := v.GetRoleID(appRolePolicy)
				eUtils.LogErrorObject(config, err, true)

				appRoleSecretID, err := v.GetSecretID(appRolePolicy)
				eUtils.LogErrorObject(config, err, true)

				fmt.Printf("Created new role id and secret id for " + appRolePolicy + ".\n")
				fmt.Printf("Role ID: %s\n", appRoleID)
				fmt.Printf("Secret ID: %s\n", appRoleSecretID)
			}

			// Store all new tokens to new appRole.
			warn, err := mod.Write("super-secrets/tokens", tokenMap, config.Log)
			eUtils.LogErrorObject(config, err, true)
			eUtils.LogWarningsObject(config, warn, true)
		}
	}

	// New vaults you can't also seed at same time
	// because you first need tokens to do so.  Only seed if !new.
	if !*newPtr {
		// Seed the vault with given seed directory
		mod, _ := helperkv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, true, logger) // Connect to vault
		if mod != nil {
			defer mod.Release()
		}
		mod.Env = *envPtr
		mod.RawEnv = eUtils.GetRawEnv(*envPtr)
		if valid, errValidateEnvironment := mod.ValidateEnvironment(mod.RawEnv, *uploadCertPtr, "", config.Log); errValidateEnvironment != nil || !valid {
			if unrestrictedValid, errValidateUnrestrictedEnvironment := mod.ValidateEnvironment(mod.RawEnv, false, "_unrestricted", config.Log); errValidateUnrestrictedEnvironment != nil || !unrestrictedValid {
				eUtils.LogAndSafeExit(config, "Mismatched token for requested environment: "+mod.Env, 1)
				return
			}
		}
		var subSectionSlice = make([]string, 0) //Assign slice with the appriopiate slice
		if len(restrictedSlice) > 0 {
			subSectionSlice = append(subSectionSlice, restrictedSlice...)
		}
		if len(indexSlice) > 0 {
			subSectionSlice = append(subSectionSlice, indexSlice...)
		}

		var filteredSectionSlice []string
		var serviceFilterSlice []string
		sectionSlice := []string{*eUtils.IndexValueFilterPtr}

		// Chewbacca: redo this next if section
		if len(*eUtils.IndexValueFilterPtr) > 0 {
			filterSlice := strings.Split(*eUtils.IndexValueFilterPtr, ",")
			for _, filter := range filterSlice {
				for _, section := range sectionSlice {
					if filter == section {
						filteredSectionSlice = append(filteredSectionSlice, section)
					}
				}
			}
			sectionSlice = filteredSectionSlice
		}
		if len(*eUtils.ServiceFilterPtr) > 0 {
			if len(sectionSlice) == 0 {
				eUtils.LogAndSafeExit(config, "No available indexes found for "+*eUtils.IndexValueFilterPtr, 1)
			}
			serviceFilterSlice = strings.Split(*eUtils.ServiceFilterPtr, ",")
			if len(*eUtils.ServiceNameFilterPtr) > 0 {
				*eUtils.ServiceNameFilterPtr = "/" + *eUtils.ServiceNameFilterPtr //added "/" - used path later
			}
		}
		if len(serviceFilterSlice) > 0 {
			serviceFilterSlice = strings.Split(*eUtils.ServiceFilterPtr, ",")
		}
		sectionKey := "/"
		if len(*eUtils.IndexValueFilterPtr) > 0 && len(*eUtils.IndexedPtr) > 0 { //*******
			if len(*eUtils.IndexedPtr) > 0 || len(*eUtils.RestrictedPtr) > 0 || len(*eUtils.ProtectedPtr) > 0 {
				if len(*eUtils.IndexedPtr) > 0 {
					sectionKey = "/Index/"
				} else if len(*eUtils.RestrictedPtr) > 0 {
					sectionKey = "/Restricted/"
				}
			}
		} else if len(*eUtils.ProtectedPtr) > 0 {
			sectionKey = "/Protected/"
		}
		var subSectionName string
		if len(*eUtils.IndexNameFilterPtr) > 0 {
			subSectionName = *eUtils.IndexNameFilterPtr
		} else {
			subSectionName = ""
		}

		var fileFilter []string
		if *nestPtr {
			fileFilter = append(fileFilter, "nest")
		}

		config = &eUtils.DriverConfig{
			Insecure:          *insecurePtr,
			Token:             v.GetToken(),
			VaultAddress:      *addrPtr,
			Env:               *envPtr,
			EnvRaw:            eUtils.GetRawEnv(*envPtr),
			SectionKey:        sectionKey,
			SectionName:       subSectionName,
			SubSectionValue:   *eUtils.IndexValueFilterPtr,
			SubSectionName:    *eUtils.ServiceNameFilterPtr,
			FileFilter:        fileFilter,
			SecretMode:        true, //  "Only override secret values in templates?"
			ProjectSections:   subSectionSlice,
			ServiceFilter:     serviceFilterSlice,
			DynamicPathFilter: *dynamicPathPtr,
			ServicesWanted:    []string{*servicePtr},
			StartDir:          append([]string{}, *seedPtr),
			EndDir:            "",
			WantCerts:         *uploadCertPtr, // TODO: this was false...
			GenAuth:           false,
			Log:               logger,
		}

		il.SeedVault(config)
	}

	logger.SetPrefix("[INIT]")
	logger.Println("=============End Vault Initialization=============")
	logger.Println()

	// Uncomment this when deployed to avoid a hanging root token
	// v.RevokeSelf()
}
