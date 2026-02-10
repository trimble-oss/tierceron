package utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	"github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/pkg/oauth"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
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

// kernelZConfig is the configuration structure for KernelZ OAuth/JWT authentication
// This is a read-only configuration file - credentials are NEVER written to it
type kernelZConfig struct {
	VaultHost         string `yaml:"vault_addr"`
	AgentEnv          string `yaml:"agent_env"`
	Deployments       string `yaml:"deployments"`
	Region            string `yaml:"region"`
	OAuthDiscoveryURL string `yaml:"oauth_discovery_url"`
	OAuthClientID     string `yaml:"oauth_client_id"`
	OAuthClientSecret string `yaml:"oauth_client_secret,omitempty"`
	OAuthCallbackPort int    `yaml:"oauth_callback_port"`
	OAuthCallbackPath string `yaml:"oauth_callback_path"`
}

// Cache for KernelZ config to avoid reading file multiple times
var (
	cachedKernelZConfig    *kernelZConfig
	cachedKernelZConfigErr error
)

// getKernelZConfig reads and caches the KernelZ config file
func getKernelZConfig(logger *log.Logger) (*kernelZConfig, error) {
	if cachedKernelZConfig != nil {
		return cachedKernelZConfig, nil
	}
	if cachedKernelZConfigErr != nil {
		return nil, cachedKernelZConfigErr
	}

	userHome, homeErr := userHome(logger)
	if homeErr != nil {
		cachedKernelZConfigErr = fmt.Errorf("failed to get user home: %w", homeErr)
		return nil, cachedKernelZConfigErr
	}

	configPath := userHome + "/.trcshrc"
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		cachedKernelZConfigErr = fmt.Errorf("config file not found: %s", configPath)
		return nil, cachedKernelZConfigErr
	}

	yamlFile, readErr := os.ReadFile(configPath)
	if readErr != nil {
		cachedKernelZConfigErr = fmt.Errorf("failed to read config file: %w", readErr)
		return nil, cachedKernelZConfigErr
	}

	var config kernelZConfig
	unmarshalErr := yaml.Unmarshal(yamlFile, &config)
	if unmarshalErr != nil {
		cachedKernelZConfigErr = fmt.Errorf("failed to parse config file: %w", unmarshalErr)
		return nil, cachedKernelZConfigErr
	}

	cachedKernelZConfig = &config
	return cachedKernelZConfig, nil
}

var prodRegions = []string{"west", "east", "ca"}

func GetSupportedProdRegions() []string {
	return prodRegions
}

func FilterSupportedRegions(driverConfig *config.DriverConfig, regions []string) []string {
	if driverConfig == nil || driverConfig.CoreConfig == nil {
		return regions
	}
	filteredRegions := []string{}

	for _, region := range regions {
		if IsRegionSupported(driverConfig, region) {
			filteredRegions = append(filteredRegions, region)
		}
	}
	return filteredRegions
}

func IsRegionSupported(driverConfig *config.DriverConfig, region string) bool {
	if driverConfig == nil || driverConfig.CoreConfig == nil {
		return true
	}

	switch region {
	case "US", "dev":
		if plugincoreopts.BuildOptions.IsPluginHardwired() {
			region = "west"
		} else {
			region = "east"
		}
	case "qa":
		if plugincoreopts.BuildOptions.IsPluginHardwired() {
			region = "east"
		} else {
			region = "west"
		}
	}

	for _, supportedRegion := range driverConfig.CoreConfig.Regions {
		if strings.HasSuffix(region, supportedRegion) {
			return true
		}
	}
	return false
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

const (
	configDir        = "/.tierceron/config.yml"
	envContextPrefix = "envContext: "
)

func GetSetEnvContext(env string, envContext string) (string, string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	// This will use env by default, if blank it will use context. If context is defined, it will replace context.
	if env == "" {
		if _, errNotExist := os.Stat(dirname + configDir); errNotExist == nil {
			file, err := os.ReadFile(dirname + configDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not read the context file due to this %s error \n", err)
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

				if err = os.WriteFile(dirname+configDir, []byte(output), 0o600); err != nil {
					return "", "", err
				}
				fmt.Fprintln(os.Stderr, "Context flag has been written out.")
				env = envContext
			} else {
				currentEnvContext := "dev"
				if strings.Index(fileContent, envContextPrefix) > 0 {
					currentEnvContext = strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
				}
				if envContext != "" {
					output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
					if err = os.WriteFile(dirname+configDir, []byte(output), 0o600); err != nil {
						return "", "", err
					}
					fmt.Fprintln(os.Stderr, "Context flag has been written out.")
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
		fmt.Fprintln(os.Stderr, "Context flag will be ignored as env is defined.")
	}
	return env, envContext, nil
}

// acquireOAuthLock acquires a file-based lock to serialize OAuth authentication across processes
// Returns the lock file descriptor and error. Caller must call releaseOAuthLock when done.
func acquireOAuthLock(logger *log.Logger) (*os.File, error) {
	userHome, err := userHome(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get user home: %w", err)
	}

	lockDir := filepath.Join(userHome, ".tierceron")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	lockFile := filepath.Join(lockDir, ".oauth_lock")
	fmt.Fprintf(os.Stderr, "Acquiring OAuth lock...\n")

	// Open or create lock file
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock with timeout
	for i := 0; i < 60; i++ { // Wait up to 60 seconds
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			fmt.Fprintf(os.Stderr, "OAuth lock acquired\n")
			return f, nil
		}
		if i == 0 {
			fmt.Fprintf(os.Stderr, "Waiting for another process to complete OAuth authentication...\n")
		}
		time.Sleep(1 * time.Second)
	}

	f.Close()
	return nil, fmt.Errorf("failed to acquire OAuth lock after 60 seconds")
}

// releaseOAuthLock releases the OAuth file lock
func releaseOAuthLock(f *os.File) {
	if f != nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		fmt.Fprintf(os.Stderr, "OAuth lock released\n")
	}
}

// oauthKernelZAuth handles OAuth/JWT authentication for KernelZ builds
// roleName parameter allows specifying which role to authenticate (e.g., "trcshhivez" or "trcshunrestricted")
// Returns: (roleID, secretID, vaultAddress, error)
func oauthKernelZAuth(driverConfig *config.DriverConfig, kzConfig *kernelZConfig, roleName string) (string, string, string, error) {
	// Check if OAuth configuration is present
	if kzConfig.OAuthDiscoveryURL == "" || kzConfig.OAuthClientID == "" {
		return "", "", "", fmt.Errorf("OAuth configuration incomplete in config.yml - need oauth_discovery_url and oauth_client_id")
	}

	// Determine which role to use (hardcoded role names)
	targetRole := roleName
	if targetRole == "" {
		targetRole = "trcshhivez"
	}

	// KernelZ: Always perform OAuth authentication (no disk caching)
	// Credentials are stored only in-memory in TokenCache for the session
	fmt.Fprintf(os.Stderr, "Performing OAuth authentication for %s...\n", targetRole)
	fmt.Fprintf(os.Stderr, "Opening browser for Identity login...\n")

	// Create vault connection
	vault, err := sys.NewVault(driverConfig.CoreConfig.Insecure, &kzConfig.VaultHost, driverConfig.CoreConfig.Env, false, false, false, driverConfig.CoreConfig.Log)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to connect to Vault: %w", err)
	}
	defer vault.Close()

	// Set default callback port and construct redirect URL
	callbackPort := kzConfig.OAuthCallbackPort
	if callbackPort == 0 {
		callbackPort = 8080
	}
	callbackPath := kzConfig.OAuthCallbackPath
	if callbackPath == "" {
		callbackPath = "/callback"
	}

	// OAuth uses its own self-contained HTTP server that starts/stops as needed
	redirectURL := fmt.Sprintf("http://localhost:%d%s", callbackPort, callbackPath)

	// Configure OAuth login with self-contained HTTP server
	oauthConfig := &sys.OAuthLoginConfig{
		OIDCDiscoveryURL: kzConfig.OAuthDiscoveryURL,
		ClientID:         kzConfig.OAuthClientID,
		ClientSecret:     kzConfig.OAuthClientSecret,
		RedirectURL:      redirectURL,
		Scopes:           []string{"openid", "profile", "email"},
		CallbackPort:     callbackPort,
		JWTRole:          targetRole,
		LocalServerConfig: &oauth.LocalServerConfig{
			Port: callbackPort,
			Path: callbackPath,
		},
	}

	// Perform OAuth login and get AppRole credentials
	ctx := context.Background()
	roleID, secretID, userInfo, err := vault.GetAppRoleCredentialsWithOAuth(ctx, oauthConfig, targetRole)
	if err != nil {
		// Return simple error message without verbose HTTP details
		return "", "", "", fmt.Errorf("authentication failed - check role authorization")
	}

	fmt.Fprintf(os.Stderr, "Successfully authenticated as: %s (%s)\n", userInfo.UserName, userInfo.UserEmail)
	fmt.Fprintf(os.Stderr, "Retrieved %s AppRole credentials\n", targetRole)

	// KernelZ: Store credentials ONLY in-memory TokenCache, never write to disk
	// This maintains security by not persisting credentials to ~/.tierceron/config.yml
	// The credentials will be used for this session only

	return roleID, secretID, kzConfig.VaultHost, nil
}

// KernelZOAuthForRole performs OAuth authentication for a specific role at runtime
// This can be called at any point to get credentials for a different role (e.g., unrestricted write access)
func KernelZOAuthForRole(driverConfig *config.DriverConfig, roleName string) error {
	if kernelopts.BuildOptions == nil || !kernelopts.BuildOptions.IsKernelZ() {
		return fmt.Errorf("KernelZ OAuth is only available in KernelZ builds")
	}

	fmt.Fprintf(os.Stderr, "KernelZOAuthForRole: received driverConfig\n")

	// Determine target role
	targetRole := roleName
	if targetRole == "" {
		targetRole = "trcshhivez"
	}

	// Use role-specific cache key to avoid mixing credentials from different roles
	cacheKey := "hivekernel:" + targetRole

	// Check if we already have valid credentials for THIS SPECIFIC role
	existingCreds := driverConfig.CoreConfig.TokenCache.GetRole(cacheKey)
	if existingCreds != nil && len(*existingCreds) == 2 && (*existingCreds)[0] != "" && (*existingCreds)[1] != "" {
		fmt.Fprintf(os.Stderr, "Using existing credentials from cache for role: %s\n", targetRole)
		// Also ensure "hivekernel" points to these credentials for backward compatibility
		driverConfig.CoreConfig.TokenCache.AddRole("hivekernel", existingCreds)
		return nil
	}

	// Read config file (cached)
	kzConfig, configErr := getKernelZConfig(driverConfig.CoreConfig.Log)
	if configErr != nil {
		return configErr
	}

	if kzConfig.OAuthDiscoveryURL == "" || kzConfig.OAuthClientID == "" {
		return fmt.Errorf("OAuth configuration not found in config.yml")
	}

	// Perform OAuth authentication for the specified role
	roleID, secretID, vaultHost, oauthErr := oauthKernelZAuth(driverConfig, kzConfig, roleName)
	if oauthErr != nil {
		// Return simple error without extra wrapping
		return oauthErr
	}

	// Store credentials under both role-specific key AND generic "hivekernel" key
	// Role-specific key prevents credential mixing between roles
	// Generic "hivekernel" key maintains backward compatibility with existing code
	appRoleSecret := []string{roleID, secretID}
	driverConfig.CoreConfig.TokenCache.AddRole(cacheKey, &appRoleSecret)
	driverConfig.CoreConfig.TokenCache.AddRole("hivekernel", &appRoleSecret)

	// Update vault address if needed
	if vaultHost != "" {
		driverConfig.CoreConfig.TokenCache.SetVaultAddress(&vaultHost)
	}

	fmt.Fprintf(os.Stderr, "Successfully authenticated for role: %s\n", targetRole)
	return nil
}

// AutoAuth attempts to authenticate a user.
func AutoAuth(driverConfig *config.DriverConfig,
	wantedTokenNamePtr *string,
	tokenProvidedPtr **string,
	envPtr *string,
	envCtxPtr *string,
	roleEntityPtr *string,
	ping bool,
) error {
	// Declare local variables
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

	// KernelZ OAuth/JWT authentication flow
	// Only trigger OAuth when specifically requesting "hivekernel" role
	// First check if we already have valid credentials in cache - skip everything if we do
	if kernelopts.BuildOptions != nil && kernelopts.BuildOptions.IsKernelZ() && RefEquals(roleEntityPtr, "hivekernel") {
		needsAuth := len((*appRoleSecret)[0]) == 0 || len((*appRoleSecret)[1]) == 0

		if needsAuth {
			// Need credentials - read config and potentially do OAuth
			kzConfig, configErr := getKernelZConfig(driverConfig.CoreConfig.Log)
			if configErr == nil && kzConfig.OAuthDiscoveryURL != "" && kzConfig.OAuthClientID != "" {
				// Acquire lock for OAuth flow
				lockFile, lockErr := acquireOAuthLock(driverConfig.CoreConfig.Log)
				if lockErr != nil {
					return fmt.Errorf("failed to acquire OAuth lock: %w", lockErr)
				}
				defer releaseOAuthLock(lockFile)

				// Double-check: another process might have completed OAuth while we waited for lock
				var existingCreds *[]string
				if roleEntityPtr != nil && *roleEntityPtr != "" {
					existingCreds = driverConfig.CoreConfig.TokenCache.GetRole(*roleEntityPtr)
				}
				if existingCreds != nil && len(*existingCreds) == 2 && (*existingCreds)[0] != "" && (*existingCreds)[1] != "" {
					roleStr := "requested role"
					if roleEntityPtr != nil && *roleEntityPtr != "" {
						roleStr = *roleEntityPtr
					}
					fmt.Fprintf(os.Stderr, "Using %s credentials from cache (populated while waiting)\n", roleStr)
					appRoleSecret = existingCreds
					if kzConfig.VaultHost != "" {
						addrPtr = &kzConfig.VaultHost
						driverConfig.CoreConfig.TokenCache.SetVaultAddress(&kzConfig.VaultHost)
					}
				} else {
					// Still need auth - perform OAuth/JWT authentication for KernelZ
					roleID, secretID, vaultHost, oauthErr := oauthKernelZAuth(driverConfig, kzConfig, "")
					if oauthErr != nil {
						return fmt.Errorf("KernelZ OAuth authentication failed: %w", oauthErr)
					}

					// Store under "hivekernel" key so OAuth-obtained credentials can be found by kernel code
					hivekernelRole := "hivekernel"
					roleEntityPtr = &hivekernelRole

					// Set AppRole credentials
					(*appRoleSecret)[0] = roleID
					(*appRoleSecret)[1] = secretID

					// Update token cache with hivekernel key (OAuth retrieved trcshhivez role)
					driverConfig.CoreConfig.TokenCache.AddRole(hivekernelRole, appRoleSecret)

					// Set vault address
					if vaultHost != "" {
						addrPtr = &vaultHost
						driverConfig.CoreConfig.TokenCache.SetVaultAddress(&vaultHost)
					}

					fmt.Fprintf(os.Stderr, "Using trcshhivez AppRole stored as hivekernel\n")
				}
			}
		}
	}

	var tokenPtr *string
	if RefLength(wantedTokenNamePtr) > 0 {
		tokenPtr = driverConfig.CoreConfig.TokenCache.GetToken(*wantedTokenNamePtr)
	}
	if tokenPtr == nil && tokenProvidedPtr != nil && RefLength(*tokenProvidedPtr) > 0 {
		if !driverConfig.CoreConfig.IsShell {
			driverConfig.CoreConfig.CurrentTokenNamePtr = wantedTokenNamePtr
		}
		tokenPtr = *tokenProvidedPtr
		// Make thebig assumption here.
		driverConfig.CoreConfig.TokenCache.AddToken(*wantedTokenNamePtr, tokenPtr)
	} else if driverConfig.CoreConfig.IsEditor &&
		RefContains(driverConfig.CoreConfig.CurrentTokenNamePtr, "unrestricted") {
		tokenPtr = driverConfig.CoreConfig.TokenCache.GetToken(*driverConfig.CoreConfig.CurrentTokenNamePtr)
		driverConfig.CoreConfig.TokenCache.AddToken(*wantedTokenNamePtr, tokenPtr)
		return nil
	}

	if RefLength(tokenPtr) != 0 &&
		!RefEquals(addrPtr, "") &&
		!RefEquals(roleEntityPtr, "deployauth") &&
		!RefEquals(roleEntityPtr, "hivekernel") &&
		!RefEquals(roleEntityPtr, "trcshhivez") &&
		!RefEquals(roleEntityPtr, "trcshunrestricted") &&
		(driverConfig.CoreConfig.CurrentTokenNamePtr == nil && wantedTokenNamePtr != nil ||
			// Accept provided token if:
			// 1. current nil, wanted not nil.
			// 2. both nil.
			// 3. both not nil and equal
			RefRefEquals(wantedTokenNamePtr, driverConfig.CoreConfig.CurrentTokenNamePtr)) {
		// For token based auth, auto auth not
		if tokenProvidedPtr != nil {
			*tokenProvidedPtr = tokenPtr
		}
		return nil
	}
	var err error

	// If cert file exists obtain secretID and appRoleID
	var cEnvCtx string
	if RefLength(roleEntityPtr) == 0 {
		roleEntityPtr = new(string)
	}

	IsCmdLineTool := driverConfig.CoreConfig.IsEditor || (!driverConfig.IsDrone && !driverConfig.CoreConfig.IsShell && (kernelopts.BuildOptions == nil || !kernelopts.BuildOptions.IsKernel()))
	IsApproleEmpty := len((*appRoleSecret)[0]) == 0 && len((*appRoleSecret)[1]) == 0

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

	if IsCmdLineTool {
		var err1 error
		fmt.Fprintf(os.Stderr, "Cmd tool auth\n")
		appRoleSecret, IsApproleEmpty, addrPtr, err1 = cmdAutoAuthHelper(appRoleSecret, IsApproleEmpty, tokenPtr, driverConfig, wantedTokenNamePtr, cEnvCtx, addrPtr, envPtr, &v, err, ping, envCtxPtr)
		if v != nil {
			defer v.Close()
		}
		if err1 != nil || ping {
			return err1
		}
	} else {
		if driverConfig == nil || driverConfig.CoreConfig == nil || !driverConfig.CoreConfig.IsEditor {
			fmt.Fprintf(os.Stderr, "No override auth connecting to vault @ %s\n", *addrPtr)
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
	}

	if len((*appRoleSecret)[0]) == 0 || len((*appRoleSecret)[1]) == 0 {
		// Vaultinit and vaultx may take this path.
		return nil
	}

	// if using appRole
	// If the wanted token name is empty, we select and appropriate default token for the role.
	if !IsApproleEmpty && RefLength(wantedTokenNamePtr) == 0 {
		fmt.Fprintf(os.Stderr, "No token name specified.  Selecting appropriate token default\n")

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
		case "trcshhivez":
			tokenNamePrefix = "trcsh_agent"
			*wantedTokenNamePtr = tokenNamePrefix + "_" + GetEnvBasis(env)
			goto skipswitch
		case "trcshunrestricted":
			tokenNamePrefix = "config"
			*wantedTokenNamePtr = tokenNamePrefix + "_" + GetEnvBasis(env) + "_unrestricted"
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
		// check that none are empty
		if len((*appRoleSecret)[1]) == 0 {
			return LogAndSafeExit(driverConfig.CoreConfig, "Missing required secretID", 1)
		} else if len((*appRoleSecret)[0]) == 0 {
			return LogAndSafeExit(driverConfig.CoreConfig, "Missing required appRoleID", 1)
		} else if *wantedTokenNamePtr == "" {
			return LogAndSafeExit(driverConfig.CoreConfig, "Missing required tokenName", 1)
		} else if *wantedTokenNamePtr == "Invalid environment" {
			return LogAndSafeExit(driverConfig.CoreConfig, "Invalid environment:"+*envPtr, 1)
		}
		// check that token matches environment
		tokenParts := strings.Split(*wantedTokenNamePtr, "_")
		tokenEnv := tokenParts[len(tokenParts)-1]
		if GetEnvBasis(env) != tokenEnv {
			return LogAndSafeExit(driverConfig.CoreConfig, "Token doesn't match environment", 1)
		}
	}

	if len(*wantedTokenNamePtr) > 0 && (RefLength(driverConfig.CoreConfig.CurrentTokenNamePtr) == 0 || !RefRefEquals(wantedTokenNamePtr, driverConfig.CoreConfig.CurrentTokenNamePtr)) {
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
		case "pubrole":
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
		case "trcshhivez":
			mod.EnvBasis = "trcshhivez"
			mod.Env = "trcshhivez"
		case "trcshunrestricted":
			mod.EnvBasis = "trcshunrestricted"
			mod.Env = "trcshunrestricted"
		case "rattan":
			mod.EnvBasis = "rattan"
			mod.Env = "rattan"
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
		if tokenProvidedPtr != nil {
			*tokenProvidedPtr = tokenPtr
		}
	}
	LogInfo(driverConfig.CoreConfig, "Auth credentials obtained.")
	return nil
}

// cmdAutoAuthHelper is a helper function to handle command line authentication.
func cmdAutoAuthHelper(appRoleSecret *[]string, IsApproleEmpty bool, tokenPtr *string, driverConfig *config.DriverConfig, wantedTokenNamePtr *string, cEnvCtx string, addrPtr *string, envPtr *string, v **sys.Vault, err error, ping bool, envCtxPtr *string) (*[]string, bool, *string, error) {
	var override bool
	var exists bool
	var vaultHost string
	var secretID string
	var approleID string

	// New values available for the cert file
	if len((*appRoleSecret)[0]) > 0 && len((*appRoleSecret)[1]) > 0 {
		override = true
	}

	// If the appRoleSecret is empty, we need to read the auth parts from cert if it exists
	if IsApproleEmpty {
		var errAuth error
		readAuthParts := !override && (RefLength(tokenPtr) == 0 ||
			!RefEquals(driverConfig.CoreConfig.CurrentTokenNamePtr, *wantedTokenNamePtr))

		exists, _, errAuth = ReadAuthParts(driverConfig, readAuthParts)
		if errAuth != nil {
			return nil, false, nil, errAuth
		} else {
			appRoleSecretFromCert := driverConfig.CoreConfig.TokenCache.GetRoleStr(driverConfig.CoreConfig.CurrentRoleEntityPtr)
			if RefLength(addrPtr) == 0 {
				addrPtr = driverConfig.CoreConfig.TokenCache.VaultAddressPtr
			}
			if appRoleSecretFromCert != nil {
				appRoleSecret = appRoleSecretFromCert
			}
		}
		// Re-evaluate
		IsApproleEmpty = len((*appRoleSecret)[0]) == 0 && len((*appRoleSecret)[1]) == 0

		if !override && !exists {
			scanner := bufio.NewScanner(os.Stdin)
			// Enter ID tokens
			if !prod.IsProd() {
				fmt.Fprintln(os.Stderr, "No cert file found, please enter config IDs")
			} else {
				fmt.Fprintln(os.Stderr, driverConfig.CoreConfig, "Please enter config IDs")
			}
			if addrPtr != nil && *addrPtr != "" {
				fmt.Fprintln(os.Stderr, "vaultHost: "+*addrPtr)
				vaultHost = *addrPtr
			} else {
				fmt.Fprint(os.Stderr, "vaultHost: ")
				scanner.Scan()
				vaultHost = scanner.Text()
			}

			if RefLength(tokenPtr) == 0 {
				if len((*appRoleSecret)[1]) > 0 {
					secretID = (*appRoleSecret)[1]
				} else {
					fmt.Fprint(os.Stderr, "secretID: ")
					scanner.Scan()
					secretID = scanner.Text()
					(*appRoleSecret)[1] = secretID
				}

				if len((*appRoleSecret)[0]) > 0 {
					secretID = (*appRoleSecret)[1]
				} else {
					fmt.Fprint(os.Stderr, "approleID: ")
					scanner.Scan()
					approleID = scanner.Text()
					(*appRoleSecret)[0] = approleID
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
				return nil, false, nil, err
			}
		}
		if envPtr != nil {
			fmt.Fprintf(os.Stderr, "Auth connecting to vault @ %s\n", *addrPtr)
			*v, err = sys.NewVault(driverConfig.CoreConfig.Insecure, addrPtr, *envPtr, false, ping, false, driverConfig.CoreConfig.Log)
		} else {
			return nil, false, nil, errors.New("envPtr is nil")
		}
		if ping {
			return nil, false, nil, nil
		}
		if err != nil {
			return nil, false, nil, err
		}

		if override || !exists {
			var dump []byte

			// Get dump
			if override && exists {
				certConfigData := "vaultHost: " + *addrPtr + "\n"
				if len((*appRoleSecret)[0]) > 0 && len((*appRoleSecret)[1]) > 0 {
					certConfigData = certConfigData + "approleID: " + (*appRoleSecret)[0] + "\nsecretID: " + (*appRoleSecret)[1]
				}

				dump = []byte(certConfigData)
			} else {
				// Get current user's home directory
				userHome, err := userHome(driverConfig.CoreConfig.Log)
				if err != nil {
					return nil, false, nil, err
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
			if !prod.IsProd() &&
				(!override || exists) {
				// Get current user's home directory
				userHome, err := userHome(driverConfig.CoreConfig.Log)
				if err != nil {
					return nil, false, nil, err
				}
				driverConfig.CoreConfig.Log.Printf("User home directory %v ", userHome)

				// Create hidden folder
				if _, err := os.Stat(userHome + "/.tierceron"); os.IsNotExist(err) {
					err = os.MkdirAll(userHome+"/.tierceron", 0o700)
					if err != nil {
						return nil, false, nil, err
					}
				}

				// Create cert file
				writeErr := os.WriteFile(userHome+"/.tierceron/config.yml", dump, 0o600)
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
		}
	}
	return appRoleSecret, IsApproleEmpty, addrPtr, nil
}

func ReadAuthParts(driverConfig *config.DriverConfig,
	readAuthParts bool,
) (bool, string, error) {
	exists := false
	var c cert
	if !prod.IsProd() {
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
