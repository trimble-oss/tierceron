package server

import (
	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	"fmt"
	"strings"
)

func (s *Server) getTemplateData() (map[string]interface{}, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr, s.CertPath)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	envList, err := mod.List("verification")
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	if envs, ok := envList.Data["keys"].([]interface{}); ok {
		environments := map[string]interface{}{}
		for _, env := range envs {
			if mod.Env, ok = env.(string); ok {
				services := map[string]interface{}{}
				servicePaths, err := s.getPaths(mod, "templates/")
				if err != nil {
					utils.LogErrorObject(err, s.Log, false)
					return nil, err
				}
				for _, servicePath := range servicePaths {
					files := map[string][]*Value{}
					filePaths, err := s.getPaths(mod, servicePath)
					if err != nil {
						utils.LogErrorObject(err, s.Log, false)
						return nil, err
					}

					for _, filePath := range filePaths {
						// Skip directories containing just the template file
						if filePath[len(filePath)-1] == '/' {
							continue
						}
						kvs, err := mod.ReadData(filePath)
						secrets := []*Value{}
						if err != nil {
							return nil, err
						}
						for k, v := range kvs {
							// Get path to secret
							if val, ok := v.([]interface{}); ok {
								if path, ok := val[0].(string); ok {
									path := strings.SplitN(path, "/", 2)[1]
									validity, err := mod.ReadData("verification/" + path)
									if err != nil {
										return nil, err
									}
									if valid, ok := validity["verified"].(bool); ok && valid {
										secrets = append(secrets, &Value{key: k, value: "true", source: "templates"})
									} else {
										secrets = append(secrets, &Value{key: k, value: "false", source: "templates"})
									}
								}
							}
						}
						files[getPathEnd(filePath)] = secrets
					}
					services[getPathEnd(servicePath)] = files
				}
				envName := mod.Env[:len(mod.Env)-1]
				environments[string(envName)] = services
			}
		}
		return environments, nil
	}

	return nil, fmt.Errorf("Error getting paths")
}
