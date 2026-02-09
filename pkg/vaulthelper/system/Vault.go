package system

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v2"
)

// Vault Represents a vault connection for managing the vault's properties
type Vault struct {
	httpClient *http.Client // Handle to http client.
	client     *api.Client  // Client connected to vault
	shards     []string     // Master key shards used to unseal vault
}

// KeyTokenWrapper Contains the unseal keys and root token
type KeyTokenWrapper struct {
	Keys     []string // Base 64 encoded keys
	TokenPtr *string  // Root token for the vault
}

// NewVault Constructs a new vault at the given address with the given access token
func NewVault(insecure bool, addressPtr *string, env string, newVault bool, pingVault bool, scanVault bool, logger *log.Logger) (*Vault, error) {
	return NewVaultWithNonlocal(insecure,
		addressPtr,
		env,
		newVault,
		pingVault,
		scanVault,
		false, // allowNonLocal - false
		logger)
}

// NewVault Constructs a new vault at the given address with the given access token allowing insecure for non local.
func NewVaultWithNonlocal(insecure bool, addressPtr *string, env string, newVault bool, pingVault bool, scanVault bool, allowNonLocal bool, logger *log.Logger) (*Vault, error) {
	var httpClient *http.Client
	var err error

	if allowNonLocal {
		httpClient, err = helperkv.CreateHTTPClientAllowNonLocal(insecure, *addressPtr, env, scanVault, true)
	} else {
		httpClient, err = helperkv.CreateHTTPClient(insecure, *addressPtr, env, scanVault)
	}

	if err != nil {
		logger.Println("Connection to vault couldn't be made - vaultHost: " + *addressPtr)
		return nil, err
	}
	client, err := api.NewClient(&api.Config{Address: *addressPtr, HttpClient: httpClient})
	if err != nil {
		logger.Println("vaultHost: " + *addressPtr)
		return nil, err
	}

	health, err := client.Sys().Health()
	if err != nil {
		return nil, err
	}

	if pingVault {
		fmt.Fprintln(os.Stderr, "Ping success!")
		logger.Println("Ping success!")
		return nil, nil
	}

	if !newVault && health.Sealed {
		return nil, errors.New("Vault is sealed at " + *addressPtr)
	}

	return &Vault{
		client: client,
		shards: nil,
	}, err
}

// Confirms we have a valid and active connection to vault.  If it doesn't, it re-establishes a new connection.
func (v *Vault) RefreshClient() error {
	tries := 0
	var refreshErr error

	for tries < 3 {
		if _, err := v.GetStatus(); err != nil {
			if v.httpClient != nil {
				v.httpClient.CloseIdleConnections()
			}

			client, err := api.NewClient(&api.Config{Address: v.client.Address(), HttpClient: v.client.CloneConfig().HttpClient})
			if err != nil {
				refreshErr = err
			} else {
				v.client = client
				return nil
			}

			tries = tries + 1
		} else {
			tries = 3
		}
	}

	return refreshErr
}

// SetToken Stores the access token for this vault
func (v *Vault) SetToken(token *string) {
	v.client.SetToken(*token)
}

// GetToken Fetches current token from client
func (v *Vault) GetToken() *string {
	token := v.client.Token()
	return &token
}

// GetTokenInfo fetches data regarding this token
func (v *Vault) GetTokenInfo(tokenName string) (map[string]any, error) {
	token, err := v.client.Auth().Token().Lookup(tokenName)
	if token == nil {
		return nil, err
	}
	return token.Data, err
}

// RevokeToken If proper access given, revokes access of a token and all children
func (v *Vault) RevokeToken(token string) error {
	return v.client.Auth().Token().RevokeTree(token)
}

// RevokeSelf Revokes token of current client
func (v *Vault) RevokeSelf() error {
	return v.client.Auth().Token().RevokeSelf("")
}

// RenewSelf Renews the token associated with this vault struct
func (v *Vault) RenewSelf(increment int) error {
	_, err := v.client.Auth().Token().RenewSelf(increment)
	return err
}

// GetOrRevokeTokensInScope()
func (v *Vault) GetOrRevokeTokensInScope(dir string,
	tokenFileFiltersSet map[string]bool,
	tokenExpiration bool,
	doTidy bool,
	logger *log.Logger,
) error {
	tokenPath := dir
	tokenPolicies := []string{}

	files, err := os.ReadDir(tokenPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if len(tokenFileFiltersSet) > 0 {
			found := false

			for tokenFilter := range tokenFileFiltersSet {
				if strings.HasPrefix(f.Name(), tokenFilter) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		file, err := os.OpenFile(tokenPath+string(os.PathSeparator)+f.Name(), os.O_RDWR, 0o644)
		if file != nil {
			defer file.Close()
		}
		if err != nil {
			log.Fatal(err)
		}

		byteValue, _ := io.ReadAll(file)
		token := api.TokenCreateRequest{}
		yaml.Unmarshal(byteValue, &token)
		tokenPolicies = append(tokenPolicies, token.Policies...)
	}

	r := v.client.NewRequest("LIST", "/v1/auth/token/accessors")
	response, err := v.client.RawRequest(r)

	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		log.Fatal(err)
	}

	var jsonData map[string]any

	if err = response.DecodeJSON(&jsonData); err != nil {
		return err
	}

	if data, ok := jsonData["data"].(map[string]any); ok {
		if accessors, ok := data["keys"].([]any); ok {
			for _, accessor := range accessors {
				b := v.client.NewRequest("POST", "/v1/auth/token/lookup-accessor")

				payload := map[string]any{
					"accessor": accessor,
				}

				if err := b.SetJSONBody(payload); err != nil {
					return err
				}
				response, err := v.client.RawRequest(b)
				if response != nil && response.Body != nil {
					defer response.Body.Close()
				}

				if err != nil {
					if response.StatusCode == 403 || response.StatusCode == 400 {
						// Some accessors we don't have access to, but we don't care about those.
						continue
					} else {
						log.Fatal(err)
						return err
					}
				}
				var accessorDataMap map[string]any
				if err = response.DecodeJSON(&accessorDataMap); err != nil {
					return err
				}
				var expirationDate string
				var expirationDateOk bool
				var matchedPolicy string
				if accessorData, ok := accessorDataMap["data"].(map[string]any); ok {
					if expirationDate, expirationDateOk = accessorData["expire_time"].(string); expirationDateOk {
						currentTime := time.Now()
						expirationTime, timeError := time.Parse(time.RFC3339Nano, expirationDate)
						if timeError == nil && currentTime.Before(expirationTime) {
							if policies, ok := accessorData["policies"].([]any); ok {
								for _, policy := range policies {
									for _, tokenPolicy := range tokenPolicies {
										if policy.(string) == "root" || strings.EqualFold(policy.(string), tokenPolicy) {
											matchedPolicy = tokenPolicy
											goto revokeAccessor
										}
									}
								}
							}
						}
					}
					continue
				revokeAccessor:
					if tokenExpiration {
						fmt.Fprintln(os.Stderr, "Token with the policy "+matchedPolicy+" expires on "+expirationDate)
						continue
					} else {
						b := v.client.NewRequest("POST", "/v1/auth/token/revoke-accessor")

						payload := map[string]any{
							"accessor": accessor,
						}

						if err := b.SetJSONBody(payload); err != nil {
							return err
						}
						response, err := v.client.RawRequest(b)
						if response != nil && response.Body != nil {
							defer response.Body.Close()
						}

						if err != nil {
							log.Fatal(err)
						}

						if response.StatusCode == 204 {
							fmt.Fprintln(os.Stderr, "Revoked token with policy: "+matchedPolicy)
						} else {
							fmt.Fprintf(os.Stderr, "Failed with status: %s\n", response.Status)
							fmt.Fprintf(os.Stderr, "Failed with status code: %d\n", response.StatusCode)
							return errors.New("failure to revoke tokens")
						}
					}
				}
			}

			if !tokenExpiration && (doTidy || len(tokenFileFiltersSet) == 0) {
				b := v.client.NewRequest("POST", "/v1/auth/token/tidy")
				response, err := v.client.RawRequest(b)
				if response != nil && response.Body != nil {
					defer response.Body.Close()
				}
				if err != nil {
					log.Fatal(err)
				}

				fmt.Fprintf(os.Stderr, "Tidy success status: %s\n", response.Status)

				if response.StatusCode == 202 {
					var tidyResponseMap map[string]any
					if err = response.DecodeJSON(&tidyResponseMap); err != nil {
						return err
					}
					if warnings, ok := tidyResponseMap["warnings"].([]any); ok {
						for _, warning := range warnings {
							fmt.Fprintln(os.Stderr, warning.(string))
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "Non critical tidy success failure: %s\n", response.Status)
					fmt.Fprintf(os.Stderr, "Non critical tidy success failure: %d\n", response.StatusCode)
				}
			}
		}
	}

	return nil
}

// CreateKVPath Creates a kv engine with the specified name and description
func (v *Vault) CreateKVPath(path string, description string) error {
	return v.client.Sys().Mount(path, &api.MountInput{
		Type:        "kv",
		Description: description,
		Options:     map[string]string{"version": "2"},
	})
}

// DeleteKVPath Deletes a KV path at a specified point.
func (v *Vault) DeleteKVPath(path string) error {
	return v.client.Sys().Unmount(path)
}

// InitVault performs vault initialization and f
func (v *Vault) InitVault(keyShares int, keyThreshold int) (*KeyTokenWrapper, error) {
	request := api.InitRequest{
		SecretShares:    keyShares,
		SecretThreshold: keyThreshold,
	}

	response, err := v.client.Sys().Init(&request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "There was an error with initializing vault @ "+v.client.Address())
		return nil, err
	}
	// Remove for deployment
	fmt.Fprintln(os.Stderr, "Vault successfully Init'd")
	fmt.Fprintln(os.Stderr, "=========================")
	for _, key := range response.KeysB64 {
		fmt.Fprintf(os.Stderr, "Unseal key: %s\n", key)
	}
	fmt.Fprintf(os.Stderr, "Root token: %s\n\n", response.RootToken)

	keyToken := KeyTokenWrapper{
		Keys:     response.KeysB64,
		TokenPtr: &response.RootToken,
	}

	return &keyToken, nil
}

// GetExistsTokenRole - Gets the token role by token role name.
func (v *Vault) GetExistsTokenRoleFromFile(filename string) (bool, error) {
	roleFile, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}

	tokenRole := YamlNewTokenRoleOptions{}
	yamlErr := yaml.Unmarshal(roleFile, &tokenRole)
	if yamlErr != nil {
		return false, yamlErr
	}

	fmt.Fprintf(os.Stderr, "Role: %s\n", tokenRole.RoleName)

	r := v.client.NewRequest("GET", fmt.Sprintf("/v1/auth/token/roles/%s", tokenRole.RoleName))
	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return false, err
	}

	var jsonData map[string]any
	if err = response.DecodeJSON(&jsonData); err != nil {
		return false, err
	}

	if _, ok := jsonData["data"].(map[string]any); ok {
		return true, nil
	}

	return false, fmt.Errorf("error parsing resonse for key 'data'")
}

// CreatePolicyFromFile Creates a policy with the given name and rules
func (v *Vault) GetExistsPolicyFromFileName(filename string) (bool, error) {
	filenameParts := strings.Split(filename, ".")

	policyContent, err := v.client.Sys().GetPolicy(filenameParts[0])

	if policyContent == "" {
		return false, nil
	} else if err != nil {
		return true, err
	} else {
		return true, nil
	}
}

// CreatePolicyFromFile Creates a policy with the given name and rules
func (v *Vault) CreatePolicyFromFile(name string, filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return v.client.Sys().PutPolicy(name, string(data))
}

// CreateEmptyPolicy Creates a policy with no permissions
func (v *Vault) CreateEmptyPolicy(name string) error {
	return v.client.Sys().PutPolicy(name, "")
}

// ValidateEnvironment Ensures token has access to requested data.
func (v *Vault) ValidateEnvironment(environment string) bool {
	secret, err := v.client.Auth().Token().LookupSelf()
	valid := false
	if err == nil {
		policies, _ := secret.TokenPolicies()

		for _, policy := range policies {
			if strings.Contains(policy, environment) {
				valid = true
			}
		}

	}
	return valid
}

// SetShards Sets known shards used by this vault for unsealing
func (v *Vault) SetShards(shards []string) {
	v.shards = shards
}

// AddShard Adds a single shard to the list of shards
func (v *Vault) AddShard(shard string) {
	v.shards = append(v.shards, shard)
}

// Unseal Performs an unseal wuth this vault's shard. Returns true if unseal is successful
func (v *Vault) Unseal() (int, int, bool, error) {
	var status *api.SealStatusResponse
	var err error
	for _, shard := range v.shards {
		status, err = v.client.Sys().Unseal(shard)
		if err != nil {
			return 0, 0, false, err
		}
	}
	return status.Progress, status.T, status.Sealed, nil
}

// CreateTokenCidrRoleFromFile Creates a new token cidr role from the given file and returns the name
func (v *Vault) CreateTokenCidrRoleFromFile(filename string) error {
	tokenfile, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	tokenRole := YamlNewTokenRoleOptions{}
	yamlErr := yaml.Unmarshal(tokenfile, &tokenRole)
	if yamlErr != nil {
		return yamlErr
	}
	errRoleCreate := v.CreateNewTokenCidrRole(&tokenRole)
	return errRoleCreate
}

// CreateTokenFromFile Creates a new token from the given file and returns the name
func (v *Vault) CreateTokenFromFile(filename string) (string, error) {
	tokenfile, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	token := api.TokenCreateRequest{}
	yaml.Unmarshal(tokenfile, &token)

	tokenRole := YamlNewTokenRoleOptions{}
	yamlErr := yaml.Unmarshal(tokenfile, &tokenRole)
	if yamlErr == nil {
		if tokenRole.RoleName != "" {
			response, err := v.client.Auth().Token().CreateWithRole(&token, tokenRole.RoleName)
			if err != nil {
				return "", err
			}
			return response.Auth.ClientToken, err
		}
	}

	response, err := v.client.Auth().Token().Create(&token)
	if err != nil {
		return "", err
	}
	return response.Auth.ClientToken, err
}

// CreateTokenFromMap takes a map and generates a vault token, returning the token
func (v *Vault) CreateTokenFromMap(data map[string]any) (string, error) {
	token := &api.TokenCreateRequest{}

	// Parse input data and create request
	if policies, ok := data["policies"].([]any); ok {
		newPolicies := []string{}
		for _, p := range policies {
			if newP, ok := p.(string); ok {
				newPolicies = append(newPolicies, newP)
			}
		}
		token.Policies = newPolicies
	}
	if meta, ok := data["meta"].(map[string]string); ok {
		token.Metadata = meta
	}
	if TTL, ok := data["creation_ttl"].(string); ok {
		token.TTL = TTL
	}
	if exTTL, ok := data["explicit_max_ttl"].(string); ok {
		token.ExplicitMaxTTL = exTTL
	}
	if period, ok := data["period"].(string); ok {
		token.Period = period
	}
	if noParent, ok := data["no_parent"].(bool); ok {
		token.NoParent = noParent
	}
	if noDefault, ok := data["no_default_policy"].(bool); ok {
		token.NoDefaultPolicy = noDefault
	}
	if displayName, ok := data["display_name"].(string); ok {
		token.DisplayName = displayName
	}
	if numUses, ok := data["num_uses"].(int); ok {
		token.NumUses = numUses
	}
	if renewable, ok := data["renewable"].(bool); ok {
		token.Renewable = &renewable
	}

	response, err := v.client.Auth().Token().Create(token)
	if response == nil {
		return "", err
	}
	return response.Auth.ClientToken, err
}

// GetStatus checks the health of the vault and retrieves version and status of init/seal
func (v *Vault) GetStatus() (map[string]any, error) {
	health, err := v.client.Sys().Health()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"initialized": health.Initialized,
		"sealed":      health.Sealed,
		"version":     health.Version,
	}, nil
}

// JWTLogin authenticates to Vault using a JWT token (from OIDC) and returns a client token
func (v *Vault) JWTLogin(jwt string, role string) (*string, error) {
	r := v.client.NewRequest("POST", fmt.Sprintf("/v1/auth/jwt/login"))

	payload := map[string]any{
		"jwt":  jwt,
		"role": role,
	}

	if err := r.SetJSONBody(payload); err != nil {
		return nil, err
	}

	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	var jsonData map[string]any
	if err = response.DecodeJSON(&jsonData); err != nil {
		return nil, err
	}

	if authData, ok := jsonData["auth"].(map[string]any); ok {
		if token, ok := authData["client_token"].(string); ok {
			return &token, nil
		}
		return nil, fmt.Errorf("error parsing response for key 'auth.client_token'")
	}

	return nil, fmt.Errorf("error parsing response for key 'auth'")
}

// EnableJWT enables the JWT/OIDC auth method
func (v *Vault) EnableJWT() error {
	sys := v.client.Sys()
	err := sys.EnableAuthWithOptions("jwt", &api.EnableAuthOptions{
		Type:        "jwt",
		Description: "JWT/OIDC authentication",
	})
	return err
}

// JWTConfigOptions holds configuration for JWT/OIDC auth method
type JWTConfigOptions struct {
	OIDCDiscoveryURL string            `json:"oidc_discovery_url,omitempty"`
	OIDCClientID     string            `json:"oidc_client_id,omitempty"`
	OIDCClientSecret string            `json:"oidc_client_secret,omitempty"`
	DefaultRole      string            `json:"default_role,omitempty"`
	BoundIssuer      string            `json:"bound_issuer,omitempty"`
	JWKSCACert       string            `json:"jwks_ca_cert,omitempty"`
	ProviderConfig   map[string]string `json:"provider_config,omitempty"`
}

// ConfigureJWT configures the JWT/OIDC auth method with OIDC discovery
func (v *Vault) ConfigureJWT(options *JWTConfigOptions) error {
	r := v.client.NewRequest("POST", "/v1/auth/jwt/config")
	if err := r.SetJSONBody(options); err != nil {
		return err
	}

	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	return err
}

// JWTRoleOptions holds configuration for a JWT role
type JWTRoleOptions struct {
	RoleType        string                 `json:"role_type,omitempty"` // "jwt" or "oidc"
	BoundAudiences  []string               `json:"bound_audiences,omitempty"`
	BoundSubject    string                 `json:"bound_subject,omitempty"`
	BoundClaims     map[string]interface{} `json:"bound_claims,omitempty"`
	ClaimMappings   map[string]string      `json:"claim_mappings,omitempty"`
	UserClaim       string                 `json:"user_claim,omitempty"`
	GroupsClaim     string                 `json:"groups_claim,omitempty"`
	Policies        []string               `json:"policies,omitempty"`
	TTL             string                 `json:"ttl,omitempty"`
	MaxTTL          string                 `json:"max_ttl,omitempty"`
	Period          string                 `json:"period,omitempty"`
	TokenBoundCIDRs []string               `json:"token_bound_cidrs,omitempty"`
}

// CreateJWTRole creates a new JWT role with the given options
func (v *Vault) CreateJWTRole(roleName string, options *JWTRoleOptions) error {
	r := v.client.NewRequest("POST", fmt.Sprintf("/v1/auth/jwt/role/%s", roleName))
	if err := r.SetJSONBody(options); err != nil {
		return err
	}

	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	return err
}

// DeleteJWTRole deletes a JWT role
func (v *Vault) DeleteJWTRole(roleName string) error {
	r := v.client.NewRequest("DELETE", fmt.Sprintf("/v1/auth/jwt/role/%s", roleName))

	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	return err
}

// Proper shutdown of modifier.
func (v *Vault) Close() {
	if v.httpClient != nil {
		v.httpClient.CloseIdleConnections()
		v.httpClient.Transport = nil
		v.httpClient = nil
	}
}
