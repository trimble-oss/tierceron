package server

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
	pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"
)

// getTemplateData Fetches all keys listed under 'templates' substituting private values with verification
// Secret values will only be populated for environments with values for that secret group
// All template keys that reference public values will be populated with those values
func (s *Server) getTemplateData() (*pb.ValuesRes, error) {
	mod, err := helperkv.NewModifier(false, s.VaultToken, s.VaultAddr, "nonprod", nil, true, s.Log)
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}

	envStrings := SelectedEnvironment
	//Only display staging in prod mode
	for i, other := range envStrings {
		if other == "prod" {
			envStrings = append(envStrings[:i], envStrings[i+1:]...)
			break
		}
	}
	for _, e := range envStrings {
		mod.Env = "local/" + e
		userPaths, err := mod.List("values/", s.Log)
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
		projectPaths, err := s.getPaths(config, mod, "templates/")
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			return nil, err
		}
		for _, projectPath := range projectPaths {
			services := []*pb.ValuesRes_Env_Project_Service{}
			servicePaths, err := s.getPaths(config, mod, projectPath)
			if err != nil {
				eUtils.LogErrorObject(config, err, false)
				return nil, err
			}
			for _, servicePath := range servicePaths {
				files := []*pb.ValuesRes_Env_Project_Service_File{}
				filePaths, err := s.getTemplateFilePaths(config, mod, servicePath)
				if err != nil {
					eUtils.LogErrorObject(config, err, false)
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
						//Get metadata of versions for each filePath
						versions, err := mod.ReadVersionMetadata(filePath, s.Log)
						var dates []time.Time
						for _, v := range versions {
							if val, ok := v.(map[string]interface{}); ok {
								location, _ := time.LoadLocation("America/Los_Angeles")
								creationTime := fmt.Sprintf("%s", val["created_time"])
								t, _ := time.Parse(time.RFC3339, creationTime)
								t = t.In(location)
								dates = append(dates, t)
							}
						}
						sort.Slice(dates, func(i, j int) bool {
							return dates[i].Before(dates[j])
						})
						for i := range dates {
							year, month, day := dates[i].Date()
							hour, min, sec := dates[i].Clock()
							creationDate := strconv.Itoa(year) + "-" + strconv.Itoa(int(month)) + "-" + strconv.Itoa(day)
							creationHour := strconv.Itoa(hour) + ":" + strconv.Itoa(min) + ":" + strconv.Itoa(sec)
							s := []string{creationDate, creationHour}
							creationTime := strings.Join(s, " ")
							secrets = append(secrets, &pb.ValuesRes_Env_Project_Service_File_Value{Key: string(i), Value: creationTime, Source: "versions"})
						}
						// Find secrets groups in this environment
						vSecret, err := mod.List("super-secrets", s.Log)
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
