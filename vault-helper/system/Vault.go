package system

import (
	"fmt"
	"github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v2"
	"io/ioutil"
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
func NewVault(addr string) (*Vault, error) {
	client, err := api.NewClient(&api.Config{Address: addr})
	if err != nil {
		return nil, err
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

// RevokeToken If proper access given, revokes access of a token and all children
func (v *Vault) RevokeToken(token string) error {
	return v.client.Auth().Token().RevokeTree(token)
}

// RevokeSelf Revokes token of current client
func (v *Vault) RevokeSelf() error {
	return v.client.Auth().Token().RevokeSelf("")
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

// SetShards Sets known shards used by this vault for unsealing
func (v *Vault) SetShards(shards []string) {
	v.shards = shards
}

// Unseal Performs an unseal wuth this vault's shard
func (v *Vault) Unseal() error {
	for _, shard := range v.shards {
		_, err := v.client.Sys().Unseal(shard)
		if err != nil {
			return err
		}
	}
	return nil
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
