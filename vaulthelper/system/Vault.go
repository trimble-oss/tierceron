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

	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

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
	Keys  []string // Base 64 encoded keys
	Token string   // Root token for the vault
}

// NewVault Constructs a new vault at the given address with the given access token
func NewVault(insecure bool, address string, env string, newVault bool, pingVault bool, scanVault bool, logger *log.Logger) (*Vault, error) {
	return NewVaultWithNonlocal(insecure,
		address,
		env,
		newVault,
		pingVault,
		scanVault,
		false, // allowNonLocal - false
		logger)
}

// NewVault Constructs a new vault at the given address with the given access token allowing insecure for non local.
func NewVaultWithNonlocal(insecure bool, address string, env string, newVault bool, pingVault bool, scanVault bool, allowNonLocal bool, logger *log.Logger) (*Vault, error) {
	var httpClient *http.Client
	var err error

	if allowNonLocal {
		httpClient, err = helperkv.CreateHTTPClientAllowNonLocal(insecure, address, env, scanVault, true)
	} else {
		httpClient, err = helperkv.CreateHTTPClient(insecure, address, env, scanVault)
	}

	if err != nil {
		logger.Println("Connection to vault couldn't be made - vaultHost: " + address)
		return nil, err
	}
	client, err := api.NewClient(&api.Config{Address: address, HttpClient: httpClient})
	if err != nil {
		logger.Println("vaultHost: " + address)
		return nil, err
	}

	health, err := client.Sys().Health()
	if err != nil {
		return nil, err
	}

	if pingVault {
		fmt.Println("Ping success!")
		logger.Println("Ping success!")
		return nil, nil
	}

	if !newVault && health.Sealed {
		return nil, errors.New("Vault is sealed at " + address)
	}

	return &Vault{
		client: client,
		shards: nil}, err
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
func (v *Vault) SetToken(token string) {
	v.client.SetToken(token)
}

// GetToken Fetches current token from client
func (v *Vault) GetToken() string {
	return v.client.Token()
}

// GetTokenInfo fetches data regarding this token
func (v *Vault) GetTokenInfo(tokenName string) (map[string]interface{}, error) {
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
func (v *Vault) GetOrRevokeTokensInScope(dir string, tokenFilter string, tokenExpiration bool, logger *log.Logger) error {
	var tokenPath = dir
	var tokenPolicies = []string{}

	files, err := os.ReadDir(tokenPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if tokenFilter != "" && !strings.HasPrefix(f.Name(), tokenFilter) {
			// Token doesn't match filter...  Skipping.
			continue
		}
		var file, err = os.OpenFile(tokenPath+string(os.PathSeparator)+f.Name(), os.O_RDWR, 0644)
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

	var jsonData map[string]interface{}

	if err = response.DecodeJSON(&jsonData); err != nil {
		return err
	}

	if data, ok := jsonData["data"].(map[string]interface{}); ok {
		if accessors, ok := data["keys"].([]interface{}); ok {
			for _, accessor := range accessors {
				b := v.client.NewRequest("POST", "/v1/auth/token/lookup-accessor")

				payload := map[string]interface{}{
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
					if response.StatusCode == 403 {
						// Some accessors we don't have access to, but we don't care about those.
						continue
					} else {
						log.Fatal(err)
						return err
					}
				}
				var accessorDataMap map[string]interface{}
				if err = response.DecodeJSON(&accessorDataMap); err != nil {
					return err
				}
				var expirationDate string
				var expirationDateOk bool
				var matchedPolicy string
				if accessorData, ok := accessorDataMap["data"].(map[string]interface{}); ok {
					if expirationDate, expirationDateOk = accessorData["expire_time"].(string); expirationDateOk {
						currentTime := time.Now()
						expirationTime, timeError := time.Parse(time.RFC3339Nano, expirationDate)
						if timeError == nil && currentTime.Before(expirationTime) {
							if policies, ok := accessorData["policies"].([]interface{}); ok {
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
						fmt.Println("Token with the policy " + matchedPolicy + " expires on " + expirationDate)
						continue
					} else {
						b := v.client.NewRequest("POST", "/v1/auth/token/revoke-accessor")

						payload := map[string]interface{}{
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
							fmt.Println("Revoked token with policy: " + matchedPolicy)
						} else {
							fmt.Printf("Failed with status: %s\n", response.Status)
							fmt.Printf("Failed with status code: %d\n", response.StatusCode)
							return errors.New("failure to revoke tokens")
						}
					}
				}
			}

			if !tokenExpiration {
				b := v.client.NewRequest("POST", "/v1/auth/token/tidy")
				response, err := v.client.RawRequest(b)
				if response != nil && response.Body != nil {
					defer response.Body.Close()
				}
				if err != nil {
					log.Fatal(err)
				}

				fmt.Printf("Tidy success status: %s\n", response.Status)

				if response.StatusCode == 202 {
					var tidyResponseMap map[string]interface{}
					if err = response.DecodeJSON(&tidyResponseMap); err != nil {
						return err
					}
					if warnings, ok := tidyResponseMap["warnings"].([]interface{}); ok {
						for _, warning := range warnings {
							fmt.Println(warning.(string))
						}
					}
				} else {
					fmt.Printf("Non critical tidy success failure: %s\n", response.Status)
					fmt.Printf("Non critical tidy success failure: %d\n", response.StatusCode)
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
		Options:     map[string]string{"version": "2"}})
}

// DeleteKVPath Deletes a KV path at a specified point.
func (v *Vault) DeleteKVPath(path string) error {
	return v.client.Sys().Unmount(path)
}

// InitVault performs vault initialization and f
func (v *Vault) InitVault(keyShares int, keyThreshold int) (*KeyTokenWrapper, error) {
	request := api.InitRequest{
		SecretShares:    keyShares,
		SecretThreshold: keyThreshold}

	response, err := v.client.Sys().Init(&request)
	if err != nil {
		fmt.Println("There was an error with initializing vault @ " + v.client.Address())
		return nil, err
	}
	// Remove for deployment
	fmt.Println("Vault succesfully Init'd")
	fmt.Println("=========================")
	for _, key := range response.KeysB64 {
		fmt.Printf("Unseal key: %s\n", key)
	}
	fmt.Printf("Root token: %s\n\n", response.RootToken)

	keyToken := KeyTokenWrapper{
		Keys:  response.KeysB64,
		Token: response.RootToken}

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

	fmt.Printf("Role: %s\n", tokenRole.RoleName)

	r := v.client.NewRequest("GET", fmt.Sprintf("/v1/auth/token/roles/%s", tokenRole.RoleName))
	response, err := v.client.RawRequest(r)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return false, err
	}

	var jsonData map[string]interface{}
	if err = response.DecodeJSON(&jsonData); err != nil {
		return false, err
	}

	if _, ok := jsonData["data"].(map[string]interface{}); ok {
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
func (v *Vault) CreateTokenFromMap(data map[string]interface{}) (string, error) {
	token := &api.TokenCreateRequest{}

	// Parse input data and create request
	if policies, ok := data["policies"].([]interface{}); ok {
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
func (v *Vault) GetStatus() (map[string]interface{}, error) {
	health, err := v.client.Sys().Health()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"initialized": health.Initialized,
		"sealed":      health.Sealed,
		"version":     health.Version,
	}, nil
}

// Proper shutdown of modifier.
func (v *Vault) Close() {
	if v.httpClient != nil {
		v.httpClient.CloseIdleConnections()
		v.httpClient.Transport = nil
		v.httpClient = nil
	}
}
