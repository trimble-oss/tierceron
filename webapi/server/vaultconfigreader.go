package server

import (
	"Vault.Whoville/vaulthelper/kv"
)

//GetConfig gets a configuration by env and path.
func (s *Server) GetConfig(env string, path string) (map[string]interface{}, error) {
	mod, err := kv.NewModifier(false, s.VaultToken, s.VaultAddr, env, nil)
	if err != nil {
		return nil, err
	}
	mod.Env = env
	return mod.ReadData(path)
}
