package server

import (
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
)

func (s *Server) getTemplateData() (*pb.ValuesRes, error) {
	mod, err := kv.NewModifier(s.VaultToken, s.VaultAddr)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		return nil, err
	}

	envStrings := []string{"dev", "QA"}
	for _, e := range envStrings {
		mod.Env = "local/" + e
		userPaths, err := mod.List("values/")
		if err != nil {
			return nil, err
		}
		if userPaths == nil {
			continue
		}
		if localEnvs, ok := userPaths.Data["keys"].([]interface{}); ok {
			for _, env := range localEnvs {
				envStrings = append(envStrings, strings.Trim("local/"+e+"/"+env.(string), "/"))
			}
		}
	}
	environments := []*pb.ValuesRes_Env{}
	for _, env := range envStrings {
		mod.Env = env
		services := []*pb.ValuesRes_Env_Service{}
		servicePaths, err := s.getPaths(mod, "templates/")
		if err != nil {
			utils.LogErrorObject(err, s.Log, false)
			return nil, err
		}
		for _, servicePath := range servicePaths {
			files := []*pb.ValuesRes_Env_Service_File{}
			filePaths, err := s.getTemplateFilePaths(mod, servicePath)
			// fmt.Println("template paths")
			// fmt.Println(filePaths)
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
					secrets := []*pb.ValuesRes_Env_Service_File_Value{}
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
								if valid, ok := validity["verified"].(bool); ok {
									if valid {
										secrets = append(secrets, &pb.ValuesRes_Env_Service_File_Value{Key: k, Value: "verifiedGood", Source: "templates"})
									} else {
										secrets = append(secrets, &pb.ValuesRes_Env_Service_File_Value{Key: k, Value: "verifiedBad", Source: "templates"})
									}
								} else {
									secrets = append(secrets, &pb.ValuesRes_Env_Service_File_Value{Key: k, Value: "unverified", Source: "templates"})
								}
							}
						}
					}
					files = append(files, &pb.ValuesRes_Env_Service_File{Name: getPathEnd(filePath), Values: secrets})
				}
			}
			services = append(services, &pb.ValuesRes_Env_Service{Name: getPathEnd(servicePath), Files: files})
		}
		environments = append(environments, &pb.ValuesRes_Env{Name: string(env), Services: services})
	}

	return &pb.ValuesRes{Envs: environments}, nil
}
