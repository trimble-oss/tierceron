package trcinitbase

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
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
	fmt.Fprintln(os.Stderr, "trcshinit Version: "+"1.39")
}

// This assumes that the vault is completely new, and should only be run for the purpose
// of automating setup and initial seeding

func CommonMain(envPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	uploadCertPtr *bool,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfig *config.DriverConfig,
) {
	if driverConfig == nil || driverConfig.CoreConfig == nil || driverConfig.CoreConfig.TokenCache == nil {
		driverConfig = &config.DriverConfig{
			CoreConfig: &coreconfig.CoreConfig{
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
	var updateAppRole *bool = defaultFalse()
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
		if driverConfig == nil || driverConfig.CoreConfig == nil || !driverConfig.CoreConfig.IsEditor {
			PrintVersion()
		}
		if len(argLines) == 0 {
			if kernelopts.BuildOptions.IsKernelZ() {
				argLines = append(argLines, "tinit")
			} else {
				argLines = append(argLines, "trcinit")
			}
		}
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
		updateAppRole = flagset.Bool("updateAppRole", false, "Update AppRole without rotating tokens")
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

	if driverConfig == nil || (!driverConfig.IsShellSubProcess && (driverConfig.CoreConfig == nil || !driverConfig.CoreConfig.IsEditor)) {
		args := argLines[1:]
		for i := 0; i < len(args); i++ {
			s := args[i]
			if s[0] != '-' {
				if driverConfig.CoreConfig.Log != nil {
					driverConfig.CoreConfig.Log.Printf("Wrong flag syntax: %s", s)
				}
				return
			}
		}
		parseErr := eUtils.CheckInitFlags(flagset, argLines[1:])
		// If help flag was used, print usage and return early
		if parseErr == flag.ErrHelp {
			flagset.Usage()
			return
		}
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, parseErr.Error())
			if driverConfig.CoreConfig.Log != nil {
				driverConfig.CoreConfig.Log.Println(parseErr.Error())
			}
			return
		}

		// Prints usage if no flags are specified
		if flagset.NFlag() == 0 {
			flagset.Usage()
			return
		}
	} else {
		// Check for help flag in shell mode
		for _, arg := range argLines {
			if arg == "-h" || arg == "-help" || arg == "--help" {
				flagset.Usage()
				return
			}
		}
		flagset.Parse(nil)
	}
	if eUtils.RefLength(addrPtr) > 0 {
		driverConfig.CoreConfig.TokenCache.SetVaultAddress(addrPtr)
	}

	// Security check: Block dangerous operations in shell mode (IsKernelZ)
	if kernelopts.BuildOptions.IsKernelZ() {
		if *newPtr || *initNamespace || *rotateTokens || *tokenExpiration || *updateRole || *updatePolicy || *updateAppRole {
			fmt.Fprintln(os.Stderr, "Error: -new, -initns, -rotateTokens, -tokenExpiration, -updateRole, -updatePolicy, and -updateAppRole are not available in shell mode for security reasons.")
			return
		}
	}

	var driverConfigBase *config.DriverConfig
	if driverConfig.CoreConfig.IsShell {
		driverConfigBase = driverConfig
		*insecurePtr = driverConfigBase.CoreConfig.Insecure
	} else {
		if !kernelopts.BuildOptions.IsKernelZ() && eUtils.RefLength(tokenPtr) < 5 {
			fmt.Fprintf(os.Stderr, "-token is a required parameter for trcinit\n")
			flagset.Usage()
			return
		}
		// If logging production directory does not exist and is selected log to local directory
		if _, err := os.Stat("/var/log/"); *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"init.log" && os.IsNotExist(err) {
			*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "init.log"
		}

		// Initialize logging
		f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
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
			if strings.ContainsAny(*tokenPtr, " \t\n\r") {
				if driverConfig.CoreConfig.Log != nil {
					driverConfig.CoreConfig.Log.Println("Invalid -token contains whitespace")
				}
				return
			}

			driverConfigBase.CoreConfig.TokenCache.AddToken(*tokenNamePtr, tokenPtr)

			eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
		} else if eUtils.RefLength(tokenPtr) == 0 && eUtils.RefLength(tokenNamePtr) > 0 {
			if driverConfigBase != nil && driverConfigBase.CoreConfig != nil && driverConfigBase.CoreConfig.TokenCache != nil {
				if driverConfigBase.CoreConfig.TokenCache.GetToken(*tokenNamePtr) == nil {
					if driverConfig.CoreConfig.Log != nil {
						driverConfig.CoreConfig.Log.Println("-token cannot be empty.")
					}
					return
				}
			}
		} else {
			if kernelopts.BuildOptions.IsKernelZ() {
				envBasis := eUtils.GetEnvBasis(*envPtr)
				tokenName := fmt.Sprintf("config_%s_unrestricted", envBasis)
				tokenNamePtr = &tokenName
				roleEntity := "trcshunrestricted"
				roleEntityPtr = &roleEntity
			}
		}
	}

	// indexServiceExtFilterPtr := flag.String("serviceExtFilter", "", "Specifies which nested services (or tables) to filter") //offset or database
	// indexServiceFilterPtr := flag.String("serviceFilter", "", "Specifies which services (or tables) to filter")              // Table names
	// indexNameFilterPtr := flag.String("indexFilter", "", "Specifies which index names to filter")                            // column index, table to filter.
	// indexValueFilterPtr := flag.String("indexValueFilter", "", "Specifies which index values to filter")                     // column index value to filter on.

	allowNonLocal := false

	if *namespaceVariable == "" && *newPtr {
		if driverConfigBase.CoreConfig.Log != nil {
			driverConfigBase.CoreConfig.Log.Println("Namespace (-namespace) required to initialize a new vault.")
		}
		return
	}

	if *newPtr {
		currentDir, dirErr := os.Getwd()
		if dirErr != nil {
			if driverConfigBase.CoreConfig.Log != nil {
				driverConfigBase.CoreConfig.Log.Println("Couldn not retrieve current working directory")
			}
			return
		}

		if _, err := os.Stat(currentDir + "/vault_namespaces/vault/token_files"); err != nil {
			if driverConfigBase.CoreConfig.Log != nil {
				driverConfigBase.CoreConfig.Log.Println("Could not locate token files required to initialize a new vault.")
			}
			return
		}

		if _, err := os.Stat(currentDir + "/vault_namespaces/vault/policy_files"); err != nil {
			if driverConfigBase.CoreConfig.Log != nil {
				driverConfigBase.CoreConfig.Log.Println("Could not locate policy files  required to initialize a new vault.")
			}
			return
		}
	}

	// Enter ID tokens
	if *insecurePtr {
		if isLocal, lookupErr := helperkv.IsUrlIp(*driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr); isLocal && lookupErr == nil {
			// This is fine...
			fmt.Fprintln(os.Stderr, "Initialize local vault.")
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			// Enter ID tokens
			fmt.Fprintln(os.Stderr, "Are you sure you want to connect to non local server with self signed cert(Y): ")
			scanner.Scan()
			skipVerify := scanner.Text()
			if skipVerify == "Y" || skipVerify == "y" {
				// Good to go.
				allowNonLocal = true
			} else {
				fmt.Fprintln(os.Stderr, "This is a remote host and you did not confirm allow non local.  If this is a remote host with a self signed cert, init will fail.")
				*insecurePtr = false
			}
		}
	}

	if *nestPtr {
		var input string

		fmt.Fprintf(os.Stderr, "Are you sure you want to seed nested files? [y|n]: ")
		_, err := fmt.Scanln(&input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
			return
		}
		input = strings.ToLower(input)

		if input != "y" && input != "yes" {
			fmt.Fprintln(os.Stderr, "Seeding nested files aborted")
			return
		}
	}

	if eUtils.ServiceFilterPtr != nil && len(*eUtils.ServiceFilterPtr) != 0 && eUtils.IndexNameFilterPtr != nil && len(*eUtils.IndexNameFilterPtr) == 0 && eUtils.RestrictedPtr != nil && len(*eUtils.RestrictedPtr) != 0 {
		eUtils.IndexNameFilterPtr = eUtils.ServiceFilterPtr
	}

	indexSlice := make([]string, 0) // Checks for indexed projects
	if eUtils.IndexedPtr != nil && len(*eUtils.IndexedPtr) > 0 {
		indexSlice = append(indexSlice, strings.Split(*eUtils.IndexedPtr, ",")...)
	}

	restrictedSlice := make([]string, 0) // Checks for restricted projects
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

	if *namespaceVariable == "" && !*rotateTokens && !*tokenExpiration && !*updatePolicy && !*updateRole && !*updateAppRole && !*pingPtr {
		if !driverConfigBase.CoreConfig.IsEditor {
			// Check seed folder exists - use memfs in shell mode, otherwise use os
			var err error
			if kernelopts.BuildOptions.IsKernelZ() && driverConfig != nil && driverConfig.MemFs != nil {
				_, err = driverConfig.MemFs.Stat(*seedPtr)
			} else {
				_, err = os.Stat(*seedPtr)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "Missing required seed folder: "+*seedPtr)
				return
			}
		}
	}

	if *prodPtr {
		if !strings.HasPrefix(*envPtr, "staging") && !strings.HasPrefix(*envPtr, "prod") {
			fmt.Fprintln(os.Stderr, "The prod flag can only be used with the staging or prod env.")
			flag.Usage()
			return
		}
	} else {
		if strings.HasPrefix(*envPtr, "staging") || strings.HasPrefix(*envPtr, "prod") {
			fmt.Fprintln(os.Stderr, "The prod flag should be used with the staging or prod env.")
			flag.Usage()
			return
		}
	}
	if driverConfig.CoreConfig.IsShell {
		if eUtils.RefLength(driverConfigBase.CoreConfig.CurrentRoleEntityPtr) > 0 {
			roleEntityPtr = driverConfigBase.CoreConfig.CurrentRoleEntityPtr
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
			fmt.Fprintf(os.Stderr, "Attempting to connect to insecure vault or vault with self signed certificate.  If you really wish to continue, you may add -insecure as on option.\n")
		} else if strings.Contains(err.Error(), "no such host") {
			fmt.Fprintf(os.Stderr, "failed to connect to vault - missing host")
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}

		return
	}
	if *pingPtr {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ping failure: %v\n", err)
		}
		return
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
			fmt.Fprintln(os.Stderr, "Unable to parse totalKeyShard into int")
			return
		}
		keyThreshold, err := strconv.ParseUint(*unsealShardPtr, 10, 32)
		if err != nil || keyThreshold > math.MaxInt {
			fmt.Fprintln(os.Stderr, "Unable to parse keyThreshold into int")
			return
		}
		keyToken, err := v.InitVault(int(totalKeyShard), int(keyThreshold))
		eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, true)
		v.SetToken(keyToken.TokenPtr)
		v.SetShards(keyToken.Keys)
		// check error returned by unseal
		_, _, _, err = v.Unseal()
		eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, true)
	}
	driverConfigBase.CoreConfig.Log.Printf("Successfully connected to vault at %s\n", *driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr)

	if !*newPtr && *namespaceVariable != "" && *namespaceVariable != "vault" && !(*rotateTokens || *updatePolicy || *updateRole || *updateAppRole || *tokenExpiration) {
		if *initNamespace {
			fmt.Fprintln(os.Stderr, "Creating tokens, roles, and policies.")
			policyExists, policyErr := il.GetExistsPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v)
			if policyErr != nil {
				eUtils.LogErrorObject(driverConfigBase.CoreConfig, policyErr, false)
				fmt.Fprintln(os.Stderr, "Cannot safely determine policy.")
				return
			}

			if policyExists {
				fmt.Fprintf(os.Stderr, "Policy exists for policy configurations in directory: %s.  Refusing to continue.\n", namespacePolicyConfigs)
				return
			}

			roleExists, roleErr := il.GetExistsRoles(driverConfigBase.CoreConfig, namespaceRoleConfigs, v)
			if roleErr != nil {
				eUtils.CheckError(driverConfigBase.CoreConfig, roleErr, false)
				fmt.Fprintln(os.Stderr, "Cannot safely determine role.")
			}

			if roleExists {
				fmt.Fprintf(os.Stderr, "Role exists for role configurations in directory: %s.  Refusing to continue.\n", namespaceRoleConfigs)
				return
			}

			// Special path for generating custom scoped tokens.  This path requires
			// a root token usually.
			//

			// Upload Create/Update new cidr roles.
			fmt.Fprintln(os.Stderr, "Creating role")
			il.UploadTokenCidrRoles(driverConfigBase.CoreConfig, namespaceRoleConfigs, v)
			// Upload Create/Update policies from the given policy directory

			fmt.Fprintln(os.Stderr, "Creating policy")
			il.UploadPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v, false)

			// Upload tokens from the given token directory
			fmt.Fprintln(os.Stderr, "Creating tokens")
			tokens := il.UploadTokens(driverConfigBase.CoreConfig, namespaceTokenConfigs, tokenFileFiltersSet, v)
			if len(tokens) > 0 {
				driverConfigBase.CoreConfig.Log.Println(*namespaceVariable + " tokens successfully created.")
			} else {
				driverConfigBase.CoreConfig.Log.Println(*namespaceVariable + " tokens failed to create.")
			}
			return
		} else {
			fmt.Fprintln(os.Stderr, "initns or rotateTokens required with the namespace paramater.")
			return
		}
	}

	if !*newPtr && (*updatePolicy || *rotateTokens || *tokenExpiration || *updateRole || *updateAppRole) {
		if *tokenExpiration {
			fmt.Fprintln(os.Stderr, "Checking token expiration.")
			roleId, lease, err := v.GetRoleID("bamboo")
			eUtils.CheckError(driverConfigBase.CoreConfig, err, false)
			fmt.Fprintln(os.Stderr, "AppRole id: "+roleId+" expiration is set to (zero means never expire): "+lease)
		} else {
			if *rotateTokens {
				if len(tokenFileFiltersSet) == 0 {
					fmt.Fprintln(os.Stderr, "Rotating tokens.")
				} else {
					fmt.Fprint(os.Stderr, "Adding tokens: ")
					first := true
					for tokenFilter := range tokenFileFiltersSet {
						if !first {
							fmt.Fprint(os.Stderr, ", ")
							first = false
						}
						fmt.Fprint(os.Stderr, tokenFilter)
					}
					fmt.Fprintln(os.Stderr)
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
				fmt.Fprintln(os.Stderr, "Token revocation or access failure.  Cannot continue.")
				return
			}
		}

		if *updateRole {
			// Upload Create/Update new cidr roles.
			fmt.Fprintln(os.Stderr, "Updating role")
			errTokenCidr := il.UploadTokenCidrRoles(driverConfigBase.CoreConfig, namespaceRoleConfigs, v)
			if errTokenCidr != nil {
				if *roleFileFilterPtr != "" { // If old way didn't work -> try new way.
					*rotateTokens = true
				} else {
					fmt.Fprintln(os.Stderr, "Role update failed.  Cannot continue.")
					return
				}
			} else {
				fmt.Fprintln(os.Stderr, "Role updated")
			}
		}

		if *updatePolicy {
			// Upload Create/Update policies from the given policy directory
			fmt.Fprintln(os.Stderr, "Updating policies")
			errTokenPolicy := il.UploadPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v, false)
			if errTokenPolicy != nil {
				fmt.Fprintln(os.Stderr, "Policy update failed.  Cannot continue.")
				return
			} else {
				fmt.Fprintln(os.Stderr, "Policies updated")
			}
		}

		if *updateAppRole {
			// Update AppRole without rotating tokens
			fmt.Fprintln(os.Stderr, "Updating AppRole")

			// Security audit: Check for policies that might grant unintended access
			fmt.Fprintln(os.Stderr, "Running security audit on policies...")
			policyFiles, err := os.ReadDir(namespacePolicyConfigs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not read policy directory for audit: %v\n", err)
			} else {
				conflictingPolicies := []string{}
				for _, policyFile := range policyFiles {
					if !strings.HasSuffix(policyFile.Name(), ".hcl") {
						continue
					}
					policyPath := filepath.Join(namespacePolicyConfigs, policyFile.Name())
					content, err := os.ReadFile(policyPath)
					if err != nil {
						continue
					}
					policyContent := string(content)
					// Check for overly broad wildcards
					if strings.Contains(policyContent, `"super-secrets/*"`) ||
						strings.Contains(policyContent, `"super-secrets/data/*"`) {
						policyName := policyFile.Name()[:len(policyFile.Name())-4]
						if policyName != "admin" { // admin is expected to have full access
							conflictingPolicies = append(conflictingPolicies, policyName)
						}
					}
				}
				if len(conflictingPolicies) > 0 {
					fmt.Fprintf(os.Stderr, "ERROR: The following policies have wildcards that grant access to ALL super-secrets paths:\n")
					for _, policy := range conflictingPolicies {
						fmt.Fprintf(os.Stderr, "  - %s\n", policy)
					}
					fmt.Fprintf(os.Stderr, "These policies could potentially access trcshunrestricted tokens.\n")
					fmt.Fprintf(os.Stderr, "Fix these policies to use specific paths (e.g., super-secrets/data/<rolename>/*) before creating restricted AppRoles.\n")
					return
				} else {
					fmt.Fprintln(os.Stderr, "Security audit passed: No conflicting policy wildcards found (admin excluded)")
				}
			}

			mod, err := helperkv.NewModifier(*insecurePtr, v.GetToken(), driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr, "nonprod", nil, true, driverConfigBase.CoreConfig.Log)
			if mod != nil {
				defer mod.Release()
			}

			if err != nil {
				fmt.Fprintln(os.Stderr, "Error creating modifier.")
				eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, false)
				return
			}

			approleFilters := []string{}
			if *roleFileFilterPtr != "" {
				if strings.Contains(*roleFileFilterPtr, ",") {
					approleFilters = strings.Split(*roleFileFilterPtr, ",")
				} else {
					approleFilters = append(approleFilters, *roleFileFilterPtr)
				}
			}

			files, err := os.ReadDir(namespaceAppRolePolicies)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error reading approle_files directory.")
				eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, false)
				return
			}

			for _, file := range files {
				filename := file.Name()
				ext := filepath.Ext(filename)
				roleName := filename[0 : len(filename)-len(ext)]

				// Skip if filter specified and doesn't match
				if len(approleFilters) > 0 {
					matched := false
					for _, filter := range approleFilters {
						if strings.Contains(roleName, filter) {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}

				fileYAML, parseErr := il.ParseApproleYaml(roleName, *namespaceVariable)
				if parseErr != nil {
					fmt.Fprintf(os.Stderr, "Unable to parse approle yaml file %s, skipping.\n", roleName)
					continue
				}

				tokenPerms, okPerms := fileYAML["Token_Permissions"].(map[any]any)
				if !okPerms {
					fmt.Fprintf(os.Stderr, "Read incorrect approle token permissions from file %s, skipping.\n", roleName)
					continue
				}

				// Read tokens from JSON file in current directory
				tokenFilePath := roleName + ".json"
				tokenFileData, err := os.ReadFile(tokenFilePath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading token file %s: %v\n", tokenFilePath, err)
					fmt.Fprintf(os.Stderr, "Create a JSON file named %s with token names/values: {\"config_token_dev_unrestricted\": \"actual-token-value\", ...}\n", tokenFilePath)
					continue
				}

				var allTokensData map[string]interface{}
				if err := json.Unmarshal(tokenFileData, &allTokensData); err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing token JSON file %s: %v\n", tokenFilePath, err)
					continue
				}

				// Set modifier environment to match the new approle name for writing
				mod.EnvBasis = roleName
				mod.Env = roleName

				// Build token map with only the tokens this AppRole should have access to
				tokenMap := map[string]any{}
				validationErrors := []string{}
				expectedTokenCount := 0

				// First pass: validate all tokens before making any changes
				for tokenName, hasAccess := range tokenPerms {
					if hasAccess.(bool) {
						expectedTokenCount++
						tokenNameStr := tokenName.(string)
						if tokenValue, exists := allTokensData[tokenNameStr]; exists {
							// Validate token exists and is valid in Vault
							tokenValueStr, ok := tokenValue.(string)
							if !ok {
								validationErrors = append(validationErrors, fmt.Sprintf("Token %s has invalid value type in JSON file", tokenNameStr))
								continue
							}
							if tokenValueStr == "TODO" || tokenValueStr == "" {
								validationErrors = append(validationErrors, fmt.Sprintf("Token %s has placeholder value", tokenNameStr))
								continue
							}

							// Validate the token using LookupSelf
							testMod, testErr := helperkv.NewModifier(*insecurePtr, &tokenValueStr, driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr, "nonprod", nil, true, driverConfigBase.CoreConfig.Log)
							if testErr != nil {
								validationErrors = append(validationErrors, fmt.Sprintf("Token %s failed to create modifier: %v", tokenNameStr, testErr))
								continue
							}
							if testMod != nil {
								// Validate token by calling LookupSelf - this will fail if token is invalid or expired
								lookupErr := testMod.ValidateToken()
								testMod.Release()
								if lookupErr != nil {
									validationErrors = append(validationErrors, fmt.Sprintf("Token %s is invalid or expired: %v", tokenNameStr, lookupErr))
									continue
								}
							}

							tokenMap[tokenNameStr] = tokenValue
						} else {
							validationErrors = append(validationErrors, fmt.Sprintf("Token %s not found in JSON file", tokenNameStr))
						}
					}
				}

				// Check if all tokens validated successfully
				if len(validationErrors) > 0 {
					fmt.Fprintf(os.Stderr, "Token validation failed for AppRole %s:\n", roleName)
					for _, errMsg := range validationErrors {
						fmt.Fprintf(os.Stderr, "  - %s\n", errMsg)
					}
					fmt.Fprintf(os.Stderr, "All tokens must be valid before creating/updating AppRole. Skipping.\n")
					continue
				}

				if len(tokenMap) != expectedTokenCount {
					fmt.Fprintf(os.Stderr, "Expected %d tokens but only validated %d for AppRole %s. Skipping.\n", expectedTokenCount, len(tokenMap), roleName)
					continue
				}

				if len(tokenMap) == 0 {
					fmt.Fprintf(os.Stderr, "No tokens to assign to AppRole %s, skipping.\n", roleName)
					continue
				}

				// Delete and recreate the AppRole
				resp, role_cleanup := v.DeleteRole(roleName)
				eUtils.CheckError(driverConfigBase.CoreConfig, role_cleanup, false)

				if resp.StatusCode == 404 {
					err = v.EnableAppRole()
					eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
				}

				err = v.CreateNewRole(roleName, &sys.NewRoleOptions{
					TokenTTL:    "10m",
					TokenMaxTTL: "15m",
					Policies:    []string{roleName},
				})
				eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

				appRoleID, _, err := v.GetRoleID(roleName)
				eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

				appRoleSecretID, err := v.GetSecretID(roleName)
				eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

				// Write tokens to the AppRole's storage location (super-secrets/<rolename>/tokens)
				warn, err := mod.Write("super-secrets/tokens", tokenMap, driverConfigBase.CoreConfig.Log)
				eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
				eUtils.LogWarningsObject(driverConfigBase.CoreConfig, warn, true)

				fmt.Fprintf(os.Stderr, "Created/updated AppRole: %s\n", roleName)
				fmt.Fprintf(os.Stderr, "Role ID: %s\n", appRoleID)
				fmt.Fprintf(os.Stderr, "Secret ID: %s\n", appRoleSecretID)
				fmt.Fprintf(os.Stderr, "Assigned %d token(s) to AppRole\n", len(tokenMap))
			}

			fmt.Fprintln(os.Stderr, "AppRole update completed")
		}

		if *rotateTokens && !*tokenExpiration {
			// Guard already checked above in (*rotateTokens || *tokenExpiration) block
			var tokens []*apinator.InitResp_Token
			// Create new tokens.
			fmt.Fprintln(os.Stderr, "Creating new tokens")
			tokens = il.UploadTokens(driverConfigBase.CoreConfig, namespaceTokenConfigs, tokenFileFiltersSet, v)

			mod, err := helperkv.NewModifier(*insecurePtr, v.GetToken(), driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr, "nonprod", nil, true, driverConfigBase.CoreConfig.Log) // Connect to vault
			if mod != nil {
				defer mod.Release()
			}

			if err != nil {
				fmt.Fprintln(os.Stderr, "Error creating modifer.")
				eUtils.LogErrorObject(driverConfigBase.CoreConfig, err, false)
				return
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
				fmt.Fprintln(os.Stderr, "No approles found for namespace: "+*namespaceVariable)
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
					tokenMap := map[string]any{}
					fileYAML, parseErr := il.ParseApproleYaml(approleFile, *namespaceVariable)
					if parseErr != nil {
						fmt.Fprintln(os.Stderr, "Read parsing approle yaml file, continuing to next file.")
						eUtils.LogErrorObject(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}
					if approleName, ok := fileYAML["Approle_Name"].(string); ok {
						mod.EnvBasis = approleName
						mod.Env = approleName
					} else {
						fmt.Fprintln(os.Stderr, "Read parsing approle name from file, continuing to next file.")
						eUtils.LogErrorObject(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}

					var tokenPerms map[any]any
					if permMap, okPerms := fileYAML["Token_Permissions"].(map[any]any); okPerms {
						tokenPerms = permMap
					} else {
						fmt.Fprintln(os.Stderr, "Read parsing approle token permissions from file, continuing to next file.")
						eUtils.LogErrorObject(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}

					if len(tokenPerms) == 0 {
						fmt.Fprintln(os.Stderr, "Completely skipping "+mod.EnvBasis+" as there is no token permission for this approle.")
						continue
					}

					if len(tokenFileFiltersSet) > 0 && tokenFileFiltersSet["*"] == false {
						// Filter out tokens that
						for tokenFilter := range tokenFileFiltersSet {
							if found, ok := tokenPerms[tokenFilter].(bool); !ok && !found {
								fmt.Fprintln(os.Stderr, "Skipping "+mod.EnvBasis+" as there is no token permission for this approle.")
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
						for tokenFilter := range tokenFileFiltersSet {
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

					// Overwrite with new tokens.
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

						fmt.Fprintf(os.Stderr, "Rotated role id and secret id for %s.\n", mod.EnvBasis)
						fmt.Fprintf(os.Stderr, "Role ID: %s\n", roleID)
						fmt.Fprintf(os.Stderr, "Secret ID: %s\n", secretID)
					} else {
						fmt.Fprintf(os.Stderr, "Existing role token count: %d.\n", len(existingTokens))
						fmt.Fprintf(os.Stderr, "Adding token to role: %s.\n", mod.EnvBasis)
						fmt.Fprintf(os.Stderr, "Post add role token count: %d.\n", len(tokenMap))
					}

					// Just update tokens in approle.
					// This could be adding to an existing approle or re-adding to rotated role...
					if len(tokenMap) > 0 {
						warn, err := mod.Write("super-secrets/tokens", tokenMap, driverConfigBase.CoreConfig.Log)
						eUtils.CheckError(driverConfigBase.CoreConfig, err, true)
						eUtils.LogWarningsObject(driverConfigBase.CoreConfig, warn, true)
						fmt.Fprintln(os.Stderr, "Approle tokens refreshed/updated.")
					}
				}
			}
		}
		return
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

	// TODO: Figure out raft storage initialization for -new flag
	if *newPtr {
		mod, err := helperkv.NewModifier(*insecurePtr, v.GetToken(), driverConfigBase.CoreConfig.TokenCache.VaultAddressPtr, "nonprod", nil, true, driverConfigBase.CoreConfig.Log) // Connect to vault
		if mod != nil {
			defer mod.Release()
		}
		eUtils.CheckError(driverConfigBase.CoreConfig, err, true)

		mod.Env = "bamboo"

		if mod.Exists("values/metadata") || mod.Exists("templates/metadata") || mod.Exists("super-secrets/metadata") {
			fmt.Fprintln(os.Stderr, "Vault has been initialized already...")
			return
		}

		policyExists, err := il.GetExistsPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v)
		if policyExists || err != nil {
			fmt.Fprintf(os.Stderr, "Vault may be initialized already - Policies exists.\n")
			return
		}

		// Create secret engines
		il.CreateEngines(driverConfigBase.CoreConfig, v)
		// Upload policies from the given policy directory
		il.UploadPolicies(driverConfigBase.CoreConfig, namespacePolicyConfigs, v, false)
		// Upload tokens from the given token directory
		tokens := il.UploadTokens(driverConfigBase.CoreConfig, namespaceTokenConfigs, tokenFileFiltersSet, v)
		if !*prodPtr {
			tokenMap := map[string]any{}
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

			fmt.Fprintf(os.Stderr, "Created new role id and secret id for bamboo.\n")
			fmt.Fprintf(os.Stderr, "Role ID: %s\n", roleID)
			fmt.Fprintf(os.Stderr, "Secret ID: %s\n", secretID)

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
						fmt.Fprintln(os.Stderr, "Unable to parse approle yaml file, continuing to next file.")
						eUtils.CheckError(driverConfigBase.CoreConfig, parseErr, false)
						continue
					}
					_, okPerms := fileYAML["Token_Permissions"].(map[any]any)
					if !okPerms {
						isPolicy = false
						fmt.Fprintln(os.Stderr, "Read incorrect approle token permissions from file, continuing to next file.")
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

				fmt.Fprintf(os.Stderr, "Created new role id and secret id for "+appRolePolicy+".\n")
				fmt.Fprintf(os.Stderr, "Role ID: %s\n", appRoleID)
				fmt.Fprintf(os.Stderr, "Secret ID: %s\n", appRoleSecretID)
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
				if driverConfigBase.CoreConfig.IsEditor {
					eUtils.LogErrorMessage(driverConfigBase.CoreConfig, "Cannot save.  Invalid token.", false)
				} else {
					eUtils.LogAndSafeExit(driverConfigBase.CoreConfig, fmt.Sprintf("Mismatched token for requested environment: %s base policy: %s policy: %s", mod.Env, baseDesiredPolicy, desiredPolicy), 1)
				}
				return
			}
		}
		sectionKey := "/"
		var subSectionName string = ""
		var filteredSectionSlice []string
		var serviceFilterSlice []string
		var fileFilter []string
		subSectionSlice := []string{} // Assign slice with the appriopiate slice

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
					*eUtils.ServiceNameFilterPtr = "/" + *eUtils.ServiceNameFilterPtr // added "/" - used path later
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
			MemFs:             driverConfigBase.MemFs,
			CoreConfig: &coreconfig.CoreConfig{
				IsEditor:            driverConfigBase.CoreConfig.IsEditor,
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
