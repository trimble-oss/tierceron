package server

import (
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

// GetConfig gets a configuration by env and path.
func (s *Server) GetConfig(env string, path string) (map[string]interface{}, error) {
	mod, err := helperkv.NewModifier(false, s.VaultTokenPtr, s.VaultAddrPtr, env, nil, true, s.Log)
	if err != nil {
		return nil, err
	}
	mod.Env = env
	defer mod.Release()
	return mod.ReadData(path)
}
