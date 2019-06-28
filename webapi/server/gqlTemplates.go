package server

import (
	"fmt"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
)

// getTemplateData Fetches all keys listed under 'templates' substituting private values with verification
// Secret values will only be populated for environments with values for that secret group
// All template keys that reference public values will be populated with those values
func (s *Server) getTemplateData() (*pb.ValuesRes, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	envStrings := []string{"dev", "QA", "RQA", "itdev", "servicepack", "staging"}
	for _, e := range envStrings {
		mod.Env = "local/" + e
		userPaths, err := mod.List("values/")
		if err != nil {
			return nil, err
		}
		if userPaths == nil {
			continue
		}
		if localEnvs, ok := userPaths.Data["keys"]; ok {
			for _, env := range localEnvs.([]interface{}) {
				envStrings = append(envStrings, strings.Trim("local/"+e+"/"+env.(string), "/"))
			}
		}
	}

	environments := []*pb.ValuesRes_Env{}
	for _, env := range envStrings {
		mod.Env = env
		projects := []*pb.ValuesRes_Env_Project{}
		projectPaths, err := s.getPaths(mod, "templates/")
		if err != nil {
			utils.LogErrorObject(err, s.Log, false)
			return nil, err
		}
		for _, projectPath := range projectPaths {
			services := []*pb.ValuesRes_Env_Project_Service{}
			servicePaths, err := s.getPaths(mod, projectPath)
			if err != nil {
				utils.LogErrorObject(err, s.Log, false)
				return nil, err
			}
			for _, servicePath := range servicePaths {
				files := []*pb.ValuesRes_Env_Project_Service_File{}
				filePaths, err := s.getTemplateFilePaths(mod, servicePath)
				if err != nil {
					utils.LogErrorObject(err, s.Log, false)
					return nil, err
				}
				if len(filePaths) > 0 {
					for _, filePath := range filePaths {
						// Skip directories containing just the template file
						if filePath[len(filePath)-1] == '/' {
							continue
						}
						kvs, err := mod.ReadData(filePath)
						secrets := []*pb.ValuesRes_Env_Project_Service_File_Value{}
						if err != nil {
							return nil, err
						}

						// Find secrets groups in this environment
						vSecret, err := mod.List("super-secrets")
						if err != nil {
							return nil, err
						}
						availableSecrets := map[string]bool{}
						if vSecret == nil {
							s.Log.Println("Unable to retrieve accessible secret groups for", env)
							continue

						} else {
							// Construct a string -> bool map to track accessable environments

							if vDataKeys, ok := vSecret.Data["keys"]; ok {
								if vKeys, okKeys := vDataKeys.([]interface{}); okKeys {
									for _, k := range vKeys {
										if group, ok := k.(string); ok {
											availableSecrets[group] = true
										}
									}
								} else {
									return nil, fmt.Errorf("Unable to retrieve accessible secret groups for %s", env)
								}
							} else {
								return nil, fmt.Errorf("Unable to retrieve accessible secret groups for %s", env)
							}
						}

						for k, v := range kvs {
							// Get path to secret
							if val, ok := v.([]interface{}); ok {
								if fullPath, ok := val[0].(string); ok {
									pathBlocks := strings.SplitN(fullPath, "/", 2) // Check that environment contains secret and check verification
									if pathBlocks[0] == "super-secrets" && availableSecrets[pathBlocks[1]] {
										validity, err := mod.ReadData("verification/" + pathBlocks[1])
										if err != nil {
											return nil, err
										}
										if valid, ok := validity["verified"].(bool); ok {
											if valid {
												secrets = append(secrets, &pb.ValuesRes_Env_Project_Service_File_Value{Key: k, Value: "verifiedGood", Source: "templates"})
											} else {
												secrets = append(secrets, &pb.ValuesRes_Env_Project_Service_File_Value{Key: k, Value: "verifiedBad", Source: "templates"})
											}
										} else {
											secrets = append(secrets, &pb.ValuesRes_Env_Project_Service_File_Value{Key: k, Value: "unverified", Source: "templates"})
										}
									} else if pathBlocks[0] == "values" { // Real value, fetch and populate
										if key, ok := val[1].(string); ok {
											value, err := mod.ReadValue(fullPath, key)
											if err == nil && value != "" {
												secrets = append(secrets, &pb.ValuesRes_Env_Project_Service_File_Value{Key: k, Value: value, Source: "value"})
											}
										} else {
											continue
										}
									}
								}
							}
						}
						//if you want to add extra dirs to filename, do it here
						if len(secrets) > 0 {
							files = append(files, &pb.ValuesRes_Env_Project_Service_File{Name: getPathEnd(filePath), Values: secrets})
						}
					}
				}
				if len(files) > 0 {
					services = append(services, &pb.ValuesRes_Env_Project_Service{Name: getPathEnd(servicePath), Files: files})
				}
			}
			if len(services) > 0 {
				projects = append(projects, &pb.ValuesRes_Env_Project{Name: getPathEnd(projectPath), Services: services})
			}
		}

		if len(projects) > 0 {
			envName := strings.Trim(mod.Env, "/")
			environments = append(environments, &pb.ValuesRes_Env{Name: string(envName), Projects: projects})
		}

	}
	return &pb.ValuesRes{Envs: environments}, nil
}
