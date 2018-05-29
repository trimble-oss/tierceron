package system

import "github.com/hashicorp/vault/api"

type Vault struct {
	client *api.Client // Client connected to vault
	shard  string      // Shard of master key used to unseal
}

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

func (v *Vault) Logical() *api.Logical {
	return v.client.Logical()
}

func (v *Vault) RenewToken(increment int) error {
	_, err := v.client.Auth().Token().RenewSelf(increment)
	return err
}

func (v *Vault) CreateKVPath(path string) error {
	return v.client.Sys().Mount(path, &api.MountInput{
		Type:    "kv",
		Options: map[string]string{"Version": "1"}})
}

func (v *Vault) CreatePolicy(name string, rules string) error {
	return v.client.Sys().PutPolicy(name, rules)
}
