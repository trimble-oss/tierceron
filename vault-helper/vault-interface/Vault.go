package vault

import "github.com/hashicorp/vault/api"

type Vault struct {
	client *api.Client // Client connected to vault
	//shard  string      // Shard of master key used to unseal
	//sys    *api.Sys    // Reference for system based operations
}

func (v *Vault) NewVault(addr string, token string) (*Vault, error) {
	return nil, nil
}

func (v *Vault) Logical() (*api.Logical, error) {
	return nil, nil
}
