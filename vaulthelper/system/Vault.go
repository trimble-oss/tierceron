package system

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
	"github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v2"
)

//Vault Represents a vault connection for managing the vault's properties
type Vault struct {
	client *api.Client // Client connected to vault
	shards []string    // Master key shards used to unseal vault
}

// KeyTokenWrapper Contains the unseal keys and root token
type KeyTokenWrapper struct {
	Keys  []string // Base 64 encoded keys
	Token string   // Root token for the vault
}

// NewVault Constructs a new vault at the given address with the given access token
func NewVault(addr string, env string, newVault bool) (*Vault, error) {
	httpClient, err := kv.CreateHTTPClient(env)
	if err != nil {
		return nil, err
	}
	client, err := api.NewClient(&api.Config{Address: addr, HttpClient: httpClient})
	if err != nil {
		return nil, err
	}

	health, err := client.Sys().Health()
	if err != nil {
		return nil, err
	}

	if !newVault && health.Sealed {
		return nil, errors.New("Vault is sealed")
	}

	return &Vault{
		client: client,
		shards: nil}, err
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

// RenewTokenInScope()
func (v *Vault) RenewTokenInScope() error {
	var tokenPath = "token_files"

	var tokenPolicies = []string{}

	files, err := ioutil.ReadDir(tokenPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		var file, err = os.OpenFile(f.Name(), os.O_RDWR, 0644)
		if err != nil {
			log.Fatal(err)
		}

		defer file.Close()
		byteValue, _ := ioutil.ReadAll(file)
		token := api.TokenCreateRequest{}
		yaml.Unmarshal(byteValue, &token)

		for _, policy := range token.Policies {
			tokenPolicies = append(tokenPolicies, policy)
		}
	}

	r := v.client.NewRequest("LIST", "/v1/auth/token/accessors")
	resp, err := v.client.RawRequest(r)
	defer resp.Body.Close()

	var jsonData map[string]interface{}

	if err = resp.DecodeJSON(&jsonData); err != nil {
		return err
	}

	if data, ok := jsonData["data"].(map[string]interface{}); ok {
		if accessors, ok := data["keys"].([]string); ok {
			for _, accessor := range accessors {
				b := v.client.NewRequest("POST", "/v1/auth/token/lookup-accessor")

				payload := map[string]interface{}{
					"accessor": accessor,
				}

				if err := b.SetJSONBody(payload); err != nil {
					return err
				}
				response, err := v.client.RawRequest(b)
				defer response.Body.Close()
				var accessorDataMap map[string]interface{}
				if err = response.DecodeJSON(accessorDataMap); err != nil {
					return err
				}

				if accessorData, ok := jsonData["data"].(map[string]interface{}); ok {
					if policies, ok := accessorData["policies"].([]string); ok {

						fmt.Println(policies[0])
					}
				}

			}
		}
		return fmt.Errorf("Error parsing response for accessor list")
	}
	return nil
	//response, err := v.client.Auth().Token().Create(&token)
	//return response.Auth.ClientToken, err
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
		fmt.Println("there was an error")
		return nil, err
	}
	// Remove for deployment
	fmt.Println("Vault succesfully Init'd")
	fmt.Println("=========================")
	fmt.Printf("Unseal key: %s\n", response.KeysB64[0])
	fmt.Printf("Root token: %s\n\n", response.RootToken)

	keyToken := KeyTokenWrapper{
		Keys:  response.KeysB64,
		Token: response.RootToken}

	return &keyToken, nil
}

// CreatePolicyFromFile Creates a policy with the given name and rules
func (v *Vault) CreatePolicyFromFile(name string, filepath string) error {
	data, err := ioutil.ReadFile(filepath)
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
	tokenfile, err := ioutil.ReadFile(filename)
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
	tokenfile, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	token := api.TokenCreateRequest{}
	yaml.Unmarshal(tokenfile, &token)
	response, err := v.client.Auth().Token().Create(&token)
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

//GetStatus checks the health of the vault and retrieves version and status of init/seal
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
