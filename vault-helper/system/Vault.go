package system

import "github.com/hashicorp/vault/api"

type Vault struct {
	client *api.Client // Client connected to vault
	shard  string      // Shard of master key used to unseal
}

// NewVault Constructs a new vault at the given address with the given access token
func NewVault(addr string, token string) (*Vault, error) {
	client, err := api.NewClient(&api.Config{Address: addr})
	if err != nil {
		return nil, err
	}
	client.SetToken(token)

	return &Vault{
		client: client,
		shard:  ""}, err
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
		Options:     map[string]string{"Version": "1"}})
}

// CreatePolicy Creates a policy with the given name and rules
func (v *Vault) CreatePolicy(name string, rules string) error {
	return v.client.Sys().PutPolicy(name, rules)
}
