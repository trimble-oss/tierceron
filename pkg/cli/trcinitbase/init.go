package trcinitbase

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	il "github.com/trimble-oss/tierceron/pkg/trcinit/initlib"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	sys "github.com/trimble-oss/tierceron/pkg/vaulthelper/system"
	"github.com/trimble-oss/tierceron/trcweb/rpc/apinator"
)

func defaultFalse() *bool {
	falseVar := false
	return &falseVar
}

func defaultEmpty() *string {
	emptyVar := ""
	return &emptyVar
}

func PrintVersion() {
	fmt.Println("Version: " + "1.36")
}

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func CommonMain(envPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	uploadCertPtr *bool,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *config.DriverConfig) {

	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		driverConfig = &config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				ExitOnFailure: true,
				TokenCache:    cache.NewTokenCacheEmpty(),
			},
		}
	}

	var newPtr *bool = defaultFalse()
	var seedPtr *string = defaultEmpty()
	var shardPtr *string = defaultEmpty()
	var namespaceVariable *string = defaultEmpty()

	var logFilePtr *string = defaultEmpty()
	var servicePtr *string = defaultEmpty()
	var prodPtr *bool = defaultFalse()
	var rotateTokens *bool = defaultFalse()
	var tokenExpiration *bool = defaultFalse()
	var pingPtr *bool = defaultFalse()
	var updateRole *bool = defaultFalse()
	var updatePolicy *bool = defaultFalse()
	var initNamespace *bool = defaultFalse()
	var doTidyPtr *bool = defaultFalse()
	var insecurePtr *bool = defaultFalse()
	var keyShardPtr *string = defaultEmpty()
	var unsealShardPtr *string = defaultEmpty()
	var tokenFileFilterSlicePtr *string = defaultEmpty()
	var roleFileFilterPtr *string = defaultEmpty()
	var dynamicPathPtr *string = defaultEmpty()
	var nestPtr *bool = defaultFalse()
	var roleEntityPtr *string = defaultEmpty()
	var devPtr *bool = defaultFalse()
	var tokenPtr *string = defaultEmpty()
	var addrPtr *string = defaultEmpty()

	if flagset == nil {
		PrintVersion()
		// Restricted trcinit and trcsh
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
		flagset.String("env", "dev", "Environment to configure")
		*seedPtr = coreopts.BuildOptions.GetFolderPrefix(nil) + "_seeds"
	} else {
		newPtr = flagset.Bool("new", false, "New vault being initialized. Creates engines and requests first-time initialization")
		seedPtr = flagset.String("seeds", coreopts.BuildOptions.GetFolderPrefix(nil)+"_seeds", "Directory that contains vault seeds")
		shardPtr = flagset.String("shard", "", "Key shard used to unseal a vault that has been initialized but restarted")

		namespaceVariable = flagset.String("namespace", "vault", "name of the namespace")

		logFilePtr = flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"init.log", "Output path for log files")
		servicePtr = flagset.String("service", "", "Seeding vault with a single service")
		prodPtr = flagset.Bool("prod", false, "Prod only seeds vault with staging environment")
		rotateTokens = flagset.Bool("rotateTokens", false, "rotate tokens")
		tokenExpiration = flagset.Bool("tokenExpiration", false, "Look up Token expiration dates")
		pingPtr = flagset.Bool("ping", false, "Ping vault.")
		updateRole = flagset.Bool("updateRole", false, "Update security role")
		updatePolicy = flagset.Bool("updatePolicy", false, "Update security policy")
		initNamespace = flagset.Bool("initns", false, "Init namespace (tokens, policy, and role)")
		doTidyPtr = flagset.Bool("tidy", false, "Clean up (tidy) expired tokens")
		insecurePtr = flagset.Bool("insecure", false, "By default, every ssl connection this tool makes is verified secure.  This option allows to tool to continue with server connections considered insecure.")
		keyShardPtr = flagset.String("totalKeys", "5", "Total number of key shards to make")
		unsealShardPtr = flagset.String("unsealKeys", "3", "Number of key shards needed to unseal")
		tokenFileFilterSlicePtr = flagset.String("filter", "", "Filter files for token rotation.  Comma delimited.")
		roleFileFilterPtr = flagset.String("approle", "", "Filter files for approle rotation.")
		dynamicPathPtr = flagset.String("dynamicPath", "", "Seed a specific directory in vault.")
		nestPtr = flagset.Bool("nest", false, "Seed a specific directory in vault.")
		devPtr = flagset.Bool("dev", false, "Vault server running in dev mode (does not need to be unsealed)")
		addrPtr = flagset.String("addr", "", "API endpoint for the vault")
		tokenPtr = flagset.String("token", "", "Vault access token, only use if in dev mode or reseeding")
	}

	if driverConfig == nil || !driverConfig.IsShellSubProcess {
		args := argLines[1:]
		for i := 0; i < len(args); i++ {
			s := args[i]
			if s[0] != '-' {
				fmt.Println("Wrong flag syntax: ", s)
				os.Exit(1)
			}
		}
		eUtils.CheckInitFlags(flagset, argLines[1:])

		// Prints usage if no flags are specified
		if flagset.NFlag() == 0 {
			flagset.Usage()
			os.Exit(1)
		}
	} else {
		flagset.Parse(nil)
	}
	driverConfig.CoreConfig.TokenCache.SetVaultAddress(addrPtr)

	var driverConfigBase *config.DriverConfig
	if driverConfig.CoreConfig.IsShell {
		driverConfigBase = driverConfig
		*insecurePtr = driverConfigBase.CoreConfig.Insecure
	} else {
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"init.log" && os.IsNotExist(err) {
			*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "init.log"
		}

		// Initialize logging
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if f != nil {
			defer f.Close()
		}
		logger := log.New(f, "[INIT]", log.LstdFlags)
		logger.Println("==========Beginning Vault Initialization==========")

		driverConfigBase = driverConfig
		driverConfigBase.CoreConfig.Insecure = false
		driverConfigBase.CoreConfig.Log = logger
		if eUtils.RefLength(tokenNamePtr) == 0 && eUtils.RefLength(tokenPtr) > 0 {
			envBasis := eUtils.GetEnvBasis(*envPtr)
			tokenName := fmt.Sprintf("config_token_%s_unrestricted", envBasis)
			tokenNamePtr = &tokenName
		} else if eUtils.RefLength(tokenPtr) == 0 {
			fmt.Println("-token cannot be empty.")
			os.Exit(1)
		}

		if strings.ContainsAny(*tokenPtr, " \t\n\r") {
			fmt.Println("Invalid -token contains whitespace")
			os.Exit(1)
		}

		driverConfigBase.CoreConfig.TokenCache.AddToken(*tokenNamePtr, tokenPtr)

		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
	}

	// indexServiceExtFilterPtr := flag.String("serviceExtFilter", "", "Specifies which nested services (or tables) to filter") //offset or database
	// indexServiceFilterPtr := flag.String("serviceFilter", "", "Specifies which services (or tables) to filter")              // Table names
	// indexNameFilterPtr := flag.String("indexFilter", "", "Specifies which index names to filter")                            // column index, table to filter.
	// indexValueFilterPtr := flag.String("indexValueFilter", "", "Specifies which index values to filter")                     // column index value to filter on.

	allowNonLocal := false

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
		if isLocal, lookupErr := helperkv.IsUrlIp(*driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr); isLocal && lookupErr == nil {
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

	if eUtils.ServiceFilterPtr != nil && len(*eUtils.ServiceFilterPtr) != 0 && eUtils.IndexNameFilterPtr != nil && len(*eUtils.IndexNameFilterPtr) == 0 && eUtils.RestrictedPtr != nil && len(*eUtils.RestrictedPtr) != 0 {
		eUtils.IndexNameFilterPtr = eUtils.ServiceFilterPtr
	}

	var indexSlice = make([]string, 0) //Checks for indexed projects
	if eUtils.IndexedPtr != nil && len(*eUtils.IndexedPtr) > 0 {
		indexSlice = append(indexSlice, strings.Split(*eUtils.IndexedPtr, ",")...)
	}

	var restrictedSlice = make([]string, 0) //Checks for restricted projects
	if eUtils.RestrictedPtr != nil && len(*eUtils.RestrictedPtr) > 0 {
		restrictedSlice = append(restrictedSlice, strings.Split(*eUtils.RestrictedPtr, ",")...)
	}

	if eUtils.ProtectedPtr != nil && len(*eUtils.ProtectedPtr) > 0 {
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
	autoErr := eUtils.AutoAuth(driverConfigBase, tokenNamePtr, &tokenPtr, envPtr, envCtxPtr, roleEntityPtr, *pingPtr)
	eUtils.CheckError(driverConfigBase.CoreConfig, autoErr, true)

	if !*pingPtr && !*newPtr && eUtils.RefLength(driverConfigBase.CoreConfig.TokenCache.GetToken(*tokenNamePtr)) == 0 {
		eUtils.CheckWarning(driverConfigBase.CoreConfig, "Missing auth tokens", true)
	}

	// Create a new vault system connection
	v, err := sys.NewVaultWithNonlocal(*insecurePtr, driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr, *envPtr, *newPtr, *pingPtr, false, allowNonLocal, driverConfigBase.CoreConfig.Log)
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
	// Set up token file filters if there are any.
	var tokenFileFiltersSet map[string]bool = make(map[string]bool)
	if eUtils.RefLength(tokenFileFilterSlicePtr) > 0 {
		tokenFileFilters := strings.Split(*tokenFileFilterSlicePtr, ",")
		tokenFileFiltersSet = make(map[string]bool, len(tokenFileFilters))
		for _, tokenFileFilter := range tokenFileFilters {
			tokenFileFiltersSet[tokenFileFilter] = true
		}
	}

	eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, true)

	// Trying to use local, prompt for username/password
	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		*envPtr, err = eUtils.LoginToLocal()
		eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, true)
		driverConfigBase.CoreConfig.Log.Printf("Login successful, using local envronment: %s\n", *envPtr)
	}

	if *devPtr || !*newPtr { // Dev server, initialization taken care of, get root token
		v.SetToken(driverConfigBase.CoreConfig.TokenCache.GetToken(*tokenNamePtr))
	} else { // Unseal and grab keys/root token
		totalKeyShard, err := strconv.ParseUint(*keyShardPtr, 10, 32)
		if err != nil || totalKeyShard > math.MaxInt {
			fmt.Println("Unable to parse totalKeyShard into int")
			os.Exit(-1)
		}
		keyThreshold, err := strconv.ParseUint(*unsealShardPtr, 10, 32)
		if err != nil || keyThreshold > math.MaxInt {
			fmt.Println("Unable to parse keyThreshold into int")
			os.Exit(-1)
		}
		keyToken, err := v.InitVault(int(totalKeyShard), int(keyThreshold))
		eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, true)
		v.SetToken(keyToken.TokenPtr)
		v.SetShards(keyToken.Keys)
		//check error returned by unseal
		_, _, _, err = v.Unseal()
		eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, true)
	}
	driverConfigBase.CoreConfig.Log.Printf("Successfully connected to vault at %s\n", *driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr)

	if !*newPtr && *namespaceVariable != "" && *namespaceVariable != "vault" && !(*rotateTokens || *updatePolicy || *updateRole || *tokenExpiration) {
		if *initNamespace {
			fmt.Println("Creating tokens, roles, and policies.")
			policyExists, policyErr := il.GetExistsPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v)
			if policyErr != nil {
				eUtils.LogErrorObject(driverConfigBase.CoreConfig, policyErr, false)
				fmt.Println("Cannot safely determine policy.")
				os.Exit(-1)
			}

			if policyExists {
				fmt.Printf("Policy exists for policy configurations in directory: %s.  Refusing to continue.\n", namespacePolicyConfigs)
				os.Exit(-1)
			}

			roleExists, roleErr := il.GetExistsRoles(driverConfigBase.CoreConfig, namespaceRoleConfigs, v)
			if roleErr != nil {
				eUtils.CheckError(driverConfigBase.CoreConfig, roleErr, false)
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
			il.UploadTokenCidrRoles(driverConfigBase.CoreConfig, namespaceRoleConfigs, v)
			// Upload Create/Update policies from the given policy directory

			fmt.Println("Creating policy")
			il.UploadPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v, false)

			// Upload tokens from the given token directory
			fmt.Println("Creating tokens")
			tokens := il.UploadTokens(driverConfigBase.CoreConfig, namespaceTokenConfigs, tokenFileFiltersSet, v)
			if len(tokens) > 0 {
				driverConfigBase.CoreConfig.Log.Println(*namespaceVariable + " tokens successfully created.")
			} else {
				driverConfigBase.CoreConfig.Log.Println(*namespaceVariable + " tokens failed to create.")
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
			eUtils.CheckError(driverConfigBase.CoreConfig, err, false)
			fmt.Println("AppRole id: " + roleId + " expiration is set to (zero means never expire): " + lease)
		} else {
			if *rotateTokens {
				if len(tokenFileFiltersSet) == 0 {
					fmt.Println("Rotating tokens.")
				} else {
					fmt.Print("Adding tokens: ")
					first := true
					for tokenFilter, _ := range tokenFileFiltersSet {
						if !first {
							fmt.Print(", ")
							first = false
						}
						fmt.Print(tokenFilter)
					}
					fmt.Println()
				}
			}
		}

		if (*rotateTokens || *tokenExpiration) && (*roleFileFilterPtr == "" || len(tokenFileFiltersSet) != 0) {
			getOrRevokeError := v.GetOrRevokeTokensInScope(namespaceTokenConfigs,
				tokenFileFiltersSet,
				*tokenExpiration,
				*doTidyPtr,
				driverConfigBase.CoreConfig.Log)
			if getOrRevokeError != nil {
				fmt.Println("Token revocation or access failure.  Cannot continue.")
				os.Exit(-1)
			}
		}

		if *updateRole {
			// Upload Create/Update new cidr roles.
			fmt.Println("Updating role")
			errTokenCidr := il.UploadTokenCidrRoles(driverConfigBase.CoreConfig, namespaceRoleConfigs, v)
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
			errTokenPolicy := il.UploadPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v, false)
			if errTokenPolicy != nil {
				fmt.Println("Policy update failed.  Cannot continue.")
				os.Exit(-1)
			} else {
				fmt.Println("Policies updated")
			}
		}

		if *rotateTokens && !*tokenExpiration {
			var tokens []*apinator.InitResp_Token
			// Create new tokens.
			fmt.Println("Creating new tokens")
			tokens = il.UploadTokens(driverConfigBase.CoreConfig, namespaceTokenConfigs, tokenFileFiltersSet, v)

			mod, err := helperkv.NewModifier(*insecurePtr, v.GetToken(), driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr, "nonprod", nil, true, driverConfigBase.CoreConfig.Log) // Connect to vault
			if mod != nil {
				defer mod.Release()
			}

			if err != nil {
				fmt.Println("Error creating modifer.")
				eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, false)
				os.Exit(-1)
			}

			approleFilters := []string{}
			if *roleFileFilterPtr != "" {
				if strings.Contains(*roleFileFilterPtr, ",") {
					approleFilters = strings.Split(*roleFileFilterPtr, ",")
				} else {
					approleFilters = append(approleFilters, *roleFileFilterPtr)
				}
			}

			// Process existing approles for provided namespace
			approleFiles := il.GetApproleFileNames(driverConfigBase.CoreConfig, *namespaceVariable)
			if len(approleFiles) == 0 {
				fmt.Println("No approles found for namespace: " + *namespaceVariable)
			} else {
				for _, approleFile := range approleFiles {
					if len(approleFilters) > 0 {
						matched := false
						for _, roleFilter := range approleFilters {
							if approleFile == roleFilter {
								matched = true
								break
							}
						}
						if !matched {
							continue
						}
					}
					tokenMap := map[string]interface{}{}
					fileYAML, parseErr := il.ParseApproleYaml(approleFile, *namespaceVariable)
					if parseErr != nil {
						fmt.Println("Read parsing approle yaml file, continuing to next file.")
						eUtils.LogErrorObject(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}
					if approleName, ok := fileYAML["Approle_Name"].(string); ok {
						mod.EnvBasis = approleName
						mod.Env = approleName
					} else {
						fmt.Println("Read parsing approle name from file, continuing to next file.")
						eUtils.LogErrorObject(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}

					var tokenPerms map[interface{}]interface{}
					if permMap, okPerms := fileYAML["Token_Permissions"].(map[interface{}]interface{}); okPerms {
						tokenPerms = permMap
					} else {
						fmt.Println("Read parsing approle token permissions from file, continuing to next file.")
						eUtils.LogErrorObject(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}

					if len(tokenPerms) == 0 {
						fmt.Println("Completely skipping " + mod.EnvBasis + " as there is no token permission for this approle.")
						continue
					}

					if len(tokenFileFiltersSet) > 0 && tokenFileFiltersSet["*"] == false {
						// Filter out tokens that
						for tokenFilter, _ := range tokenFileFiltersSet {
							if found, ok := tokenPerms[tokenFilter].(bool); !ok && !found {
								fmt.Println("Skipping " + mod.EnvBasis + " as there is no token permission for this approle.")
								continue
							}
						}
					}

					existingTokens, err := mod.ReadData("super-secrets/tokens")
					hasAllFilteredTokens := true
					if err != nil {
						// TODO: consider scenarios...
					} else {
						// Check the filtering...
						for tokenFilter, _ := range tokenFileFiltersSet {
							if _, ok := existingTokens[tokenFilter].(string); !ok {
								hasAllFilteredTokens = false
								break
							}
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
					}

					//Overwrite with new tokens.
					for _, token := range tokens {
						// Everything but webapp give access by app role and secret.
						if found, ok := tokenPerms[token.Name].(bool); ok && found && token.Name != "admin" && token.Name != "webapp" {
							tokenMap[token.Name] = token.Value
						}
					}

					if *roleFileFilterPtr != "" && hasAllFilteredTokens {
						//
						// Wipe existing role.
						// Recreate the role.
						//
						resp, role_cleanup := v.DeleteRole(mod.EnvBasis)
						eUtils.CheckError(driverConfigBase.CoreConfig, role_cleanup, false)

						if resp.StatusCode == 404 {
							err = v.EnableAppRole()
							eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, true)
						}

						err = v.CreateNewRole(mod.EnvBasis, &sys.NewRoleOptions{
							TokenTTL:    "10m",
							TokenMaxTTL: "15m",
							Policies:    []string{mod.EnvBasis},
						})
						eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

						roleID, _, err := v.GetRoleID(mod.EnvBasis)
						eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

						secretID, err := v.GetSecretID(mod.EnvBasis)
						eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

						fmt.Printf("Rotated role id and secret id for %s.\n", mod.EnvBasis)
						fmt.Printf("Role ID: %s\n", roleID)
						fmt.Printf("Secret ID: %s\n", secretID)
					} else {
						fmt.Printf("Existing role token count: %d.\n", len(existingTokens))
						fmt.Printf("Adding token to role: %s.\n", mod.EnvBasis)
						fmt.Printf("Post add role token count: %d.\n", len(tokenMap))
					}

					// Just update tokens in approle.
					// This could be adding to an existing approle or re-adding to rotated role...
					if len(tokenMap) > 0 {
						warn, err := mod.Write("super-secrets/tokens", tokenMap, driverConfigBase.CoreConfig.Log)
						eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
						eUtils.LogWarningsObject(driverConfigBase.CoreConfig, warn, true)
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
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
		if !success {
			driverConfigBase.CoreConfig.Log.Printf("Vault unseal progress: %d/%d key shards\n", prog, t)
			driverConfigBase.CoreConfig.Log.Println("============End Initialization Attempt============")
		} else {
			driverConfigBase.CoreConfig.Log.Println("Vault successfully unsealed")
		}
	}

	//TODO: Figure out raft storage initialization for -new flag
	if *newPtr {
		mod, err := helperkv.NewModifier(*insecurePtr, v.GetToken(), driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr, "nonprod", nil, true, driverConfigBase.CoreConfig.Log) // Connect to vault
		if mod != nil {
			defer mod.Release()
		}
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

		mod.Env = "bamboo"

		if mod.Exists("values/metadata") || mod.Exists("templates/metadata") || mod.Exists("super-secrets/metadata") {
			fmt.Println("Vault has been initialized already...")
			os.Exit(1)
		}

		policyExists, err := il.GetExistsPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v)
		if policyExists || err != nil {
			fmt.Printf("Vault may be initialized already - Policies exists.\n")
			os.Exit(1)
		}

		// Create secret engines
		il.CreateEngines(driverConfigBase.CoreConfig, v)
		// Upload policies from the given policy directory
		il.UploadPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v, false)
		// Upload tokens from the given token directory
		tokens := il.UploadTokens(driverConfigBase.CoreConfig, namespaceTokenConfigs, tokenFileFiltersSet, v)
		if !*prodPtr {
			tokenMap := map[string]interface{}{}
			for _, token := range tokens {
				tokenMap[token.Name] = token.Value
			}

			err = v.EnableAppRole()
			eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

			err = v.CreateNewRole("bamboo", &sys.NewRoleOptions{
				TokenTTL:    "10m",
				TokenMaxTTL: "15m",
				Policies:    []string{"bamboo"},
			})
			eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

			roleID, _, err := v.GetRoleID("bamboo")
			eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

			secretID, err := v.GetSecretID("bamboo")
			eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

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
						eUtils.CheckError(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}
					_, okPerms := fileYAML["Token_Permissions"].(map[interface{}]interface{})
					if !okPerms {
						isPolicy = false
						fmt.Println("Read incorrect approle token permissions from file, continuing to next file.")
						eUtils.CheckError(driverConfigBase.CoreConfig, parseErr, false)
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
				eUtils.CheckError(driverConfigBase.CoreConfig, role_cleanup, false)

				if resp.StatusCode == 404 {
					err = v.EnableAppRole()
					eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
				}

				err = v.CreateNewRole(appRolePolicy, &sys.NewRoleOptions{
					TokenTTL:    "10m",
					TokenMaxTTL: "15m",
					Policies:    []string{appRolePolicy},
				})
				eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

				appRoleID, _, err := v.GetRoleID(appRolePolicy)
				eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

				appRoleSecretID, err := v.GetSecretID(appRolePolicy)
				eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

				fmt.Printf("Created new role id and secret id for " + appRolePolicy + ".\n")
				fmt.Printf("Role ID: %s\n", appRoleID)
				fmt.Printf("Secret ID: %s\n", appRoleSecretID)
			}

			// Store all new tokens to new appRole.
			warn, err := mod.Write("super-secrets/tokens", tokenMap, driverConfigBase.CoreConfig.Log)
			eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
			eUtils.LogWarningsObject(driverConfigBase.CoreConfig, warn, true)
		}
	}

	// New vaults you can't also seed at same time
	// because you first need tokens to do so.  Only seed if !new.
	if !*newPtr {
		// Seed the vault with given seed directory
		mod, _ := helperkv.NewModifierFromCoreConfig(
			driverConfigBase.CoreConfig,
			*tokenNamePtr, *envPtr, true) // Connect to vault
		if mod != nil {
			defer mod.Release()
		}
		mod.Env = *envPtr
		mod.EnvBasis = eUtils.GetEnvBasis(*envPtr)
		if valid, baseDesiredPolicy, errValidateEnvironment := mod.ValidateEnvironment(mod.EnvBasis, *uploadCertPtr, "", driverConfigBase.CoreConfig.Log); errValidateEnvironment != nil || !valid {
			if unrestrictedValid, desiredPolicy, errValidateUnrestrictedEnvironment := mod.ValidateEnvironment(mod.EnvBasis, false, "_unrestricted", driverConfigBase.CoreConfig.Log); errValidateUnrestrictedEnvironment != nil || !unrestrictedValid {
				eUtils.LogAndSafeExit(driverConfigBase.CoreConfig, fmt.Sprintf("Mismatched token for requested environment: %s base policy: %s policy: %s", mod.Env, baseDesiredPolicy, desiredPolicy), 1)
				return
			}
		}
		sectionKey := "/"
		var subSectionName string = ""
		var filteredSectionSlice []string
		var serviceFilterSlice []string
		var fileFilter []string
		var subSectionSlice = []string{} //Assign slice with the appriopiate slice

		if !*uploadCertPtr {
			if len(restrictedSlice) > 0 {
				subSectionSlice = append(subSectionSlice, restrictedSlice...)
			}
			if len(indexSlice) > 0 {
				subSectionSlice = append(subSectionSlice, indexSlice...)
			}

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
					eUtils.LogAndSafeExit(driverConfigBase.CoreConfig, "No available indexes found for "+*eUtils.IndexValueFilterPtr, 1)
				}
				serviceFilterSlice = strings.Split(*eUtils.ServiceFilterPtr, ",")
				if len(*eUtils.ServiceNameFilterPtr) > 0 {
					*eUtils.ServiceNameFilterPtr = "/" + *eUtils.ServiceNameFilterPtr //added "/" - used path later
				}
			}
			if len(serviceFilterSlice) > 0 {
				serviceFilterSlice = strings.Split(*eUtils.ServiceFilterPtr, ",")
			}
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
			if len(*eUtils.IndexNameFilterPtr) > 0 {
				subSectionName = *eUtils.IndexNameFilterPtr
			} else {
				subSectionName = ""
			}

			if *nestPtr {
				fileFilter = append(fileFilter, "nest")
			}

		}

		dConfig := &config.DriverConfig{
			IsShellSubProcess: driverConfigBase.IsShellSubProcess,
			CoreConfig: &core.CoreConfig{
				DynamicPathFilter:   *dynamicPathPtr,
				Insecure:            *insecurePtr,
				CurrentTokenNamePtr: driverConfigBase.CoreConfig.CurrentTokenNamePtr,
				TokenCache:          driverConfigBase.CoreConfig.TokenCache,
				Env:                 *envPtr,
				EnvBasis:            eUtils.GetEnvBasis(*envPtr),
				WantCerts:           *uploadCertPtr, // TODO: this was false...
				Log:                 driverConfigBase.CoreConfig.Log,
			},
			SectionKey:      sectionKey,
			SectionName:     subSectionName,
			FileFilter:      fileFilter,
			SecretMode:      true, //  "Only override secret values in templates?"
			ProjectSections: subSectionSlice,
			ServiceFilter:   serviceFilterSlice,
			ServicesWanted:  []string{*servicePtr},
			StartDir:        append([]string{}, *seedPtr),
			EndDir:          "",
			GenAuth:         false,
		}
		if eUtils.IndexValueFilterPtr != nil {
			dConfig.SubSectionValue = *eUtils.IndexValueFilterPtr
		}

		if eUtils.ServiceNameFilterPtr != nil {
			dConfig.SubSectionName = *eUtils.ServiceNameFilterPtr
		}

		il.SeedVault(dConfig)
	}

	driverConfigBase.CoreConfig.Log.SetPrefix("[INIT]")
	driverConfigBase.CoreConfig.Log.Println("=============End Vault Initialization=============")
	driverConfigBase.CoreConfig.Log.Println()

	// Uncomment this when deployed to avoid a hanging root token
	// v.RevokeSelf()
}
