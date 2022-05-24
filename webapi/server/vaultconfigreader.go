package server

import (
	helperkv "tierceron/vaulthelper/kv"
)

//GetConfig gets a configuration by env and path.
func (s *Server) GetConfig(env string, path string) (map[string]interface{}, error) {
	mod, err := helperkv.NewModifier(false, s.VaultToken, s.VaultAddr, env, nil, s.Log)
	if err != nil {
		return nil, err
	}
	mod.Env = env
	return mod.ReadData(path)
}
