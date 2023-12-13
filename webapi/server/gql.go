package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/utils"
	pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"

	"github.com/graphql-go/graphql"
)

// VaultVals Holds environments, used for GraphQL
type VaultVals struct {
	ID   string `json:"id"`
	Envs []Env  `json:"envs"`
}

// Env represents an environment containing multiple services
type Env struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	Projects  []Project  `json:"projects"`
	Providers []Provider `json:"providers"`
}

// Project represents a project that contains multiple files
type Project struct {
	EnvID    int       `json:"envID"`
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Services []Service `json:"files"`
}

// Service represents an service that contains multiple files
type Service struct {
	EnvID  int    `json:"envID"`
	ProjID int    `json:"projID"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Files  []File `json:"values"`
}

// File represents an individual file containing template values
type File struct {
	EnvID  int     `json:"envID"`
	ProjID int     `json:"projID"`
	ServID int     `json:"servID"`
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Values []Value `json:"values"`
}

// Value represents an individual key-value pair with source
type Value struct {
	EnvID  int    `json:"envID"`
	ProjID int    `json:"projID"`
	ServID int    `json:"servID"`
	FileID int    `json:"fileID"`
	ID     int    `json:"id"`
	Key    string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

type visitedNode struct {
	index    int
	children map[string]*visitedNode
}

// Provider represents a login session provider
type Provider struct {
	EnvID    int
	ID       int
	Name     string
	Sessions []map[string]interface{}
}

// GraphQL Accepts a GraphQL query and creates a response
func (s *Server) GraphQL(ctx context.Context, req *pb.GraphQLQuery) (*pb.GraphQLResp, error) {
	rawResult := graphql.Do(graphql.Params{
		Schema:        s.GQLSchema,
		RequestString: req.Query,
		Context:       ctx,
	})

	result := &pb.GraphQLResp{}
	resultBytes := bytes.NewBuffer(nil)
	json.NewEncoder(resultBytes).Encode(rawResult)
	json.Unmarshal(resultBytes.Bytes(), result)
	return result, nil
}

// InitGQL Initializes the GQL schema
func (s *Server) InitGQL() {
	s.Log.Println("InitGQL")
	makeVaultReq := &pb.GetValuesReq{}
	integrationSessions := map[string][]map[string]interface{}{} //
	vaultSessions := map[string][]map[string]interface{}{}       //

	// Fetch template keys and values
	vault, err := s.GetValues(context.Background(), makeVaultReq)
	config := &eUtils.DriverConfig{ExitOnFailure: false, Log: s.Log}

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		eUtils.LogWarningsObject(config, []string{"GraphQL MAY not initialized (values not added)"}, false)
		return
	}

	// Fetch secret keys and verification info
	templates, err := s.getTemplateData()
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		eUtils.LogWarningsObject(config, []string{"GraphQL MAY not initialized (secrets not added)"}, false)
		return
	}

	envStrings := SelectedEnvironment
	for _, e := range envStrings { //Not including itdev and servicepack
		// Get spectrum sessions
		integrationSessions[e], err = s.getActiveSessions(config, e)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			eUtils.LogWarningsObject(config, []string{fmt.Sprintf("GraphQL MAY not initialized (Spectrum %s sessions not added)", e)}, false)
		}
		vaultSessions[e], err = s.getVaultSessions(e)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			eUtils.LogWarningsObject(config, []string{fmt.Sprintf("GraphQL MAY not initialized (Vault %s sessions not added)", e)}, false)
		}
	}

	// Merge data into one nested structure
	envIndices := map[string]*visitedNode{} // Track indices of environments in list
	envList := []Env{}

	// Environments
	for _, env := range append(vault.Envs, templates.Envs...) {
		var envQL *Env
		// Determine if this environment already exists
		if envIndices[env.Name] != nil { // Get a reference to the existing environment
			index := envIndices[env.Name].index
			envQL = &envList[index]
		} else { // Create a new environment
			envIndices[env.Name] = &visitedNode{index: len(envList), children: map[string]*visitedNode{}}
			envList = append(envList, Env{ID: len(envList), Name: env.Name, Projects: []Project{}})
			envQL = &envList[len(envList)-1]
		}
		//envQL is env at index

		projectIndices := envIndices[env.Name].children // Track indices of projects in list
		projectList := append([]Project{}, envQL.Projects...)
		// project
		for _, project := range env.Projects {
			var projectQL *Project
			if projectIndices[project.Name] != nil { // Get a reference to the existing project
				index := projectIndices[project.Name].index
				projectQL = &projectList[index]
			} else { // Create a new project
				projectIndices[project.Name] = &visitedNode{index: len(projectList), children: map[string]*visitedNode{}}
				projectList = append(projectList, Project{ID: len(projectList), EnvID: envQL.ID, Name: project.Name, Services: []Service{}})
				projectQL = &projectList[len(projectList)-1]
			}
			serviceIndices := projectIndices[project.Name].children // Track indices of services in list
			serviceList := append([]Service{}, projectQL.Services...)
			// Project
			for _, service := range project.Services {
				var serviceQL *Service
				if serviceIndices[service.Name] != nil { // Get a reference to the existing serice
					index := serviceIndices[service.Name].index
					serviceQL = &serviceList[index]
				} else { // Create a new serice
					serviceIndices[service.Name] = &visitedNode{index: len(serviceList), children: map[string]*visitedNode{}}
					serviceList = append(serviceList, Service{ID: len(serviceList), EnvID: envQL.ID, ProjID: projectQL.ID, Name: service.Name, Files: []File{}})
					serviceQL = &serviceList[len(serviceList)-1]
				}
				// Files
				fileIndices := serviceIndices[service.Name].children
				fileList := append([]File{}, serviceQL.Files...)
				for _, file := range service.Files {
					var fileQL *File
					if fileIndices[file.Name] != nil { // Get a reference to the existing file
						index := fileIndices[file.Name].index
						fileQL = &fileList[index]
					} else { // Create a new file
						fileIndices[file.Name] = &visitedNode{index: len(fileList), children: map[string]*visitedNode{}}
						fileList = append(fileList, File{ID: len(fileList), EnvID: envQL.ID, ProjID: projectQL.ID, ServID: serviceQL.ID, Name: file.Name, Values: []Value{}})
						fileQL = &fileList[len(fileList)-1]
					}
					// Values
					valList := fileQL.Values
					for _, val := range file.Values {
						valQL := Value{ID: len(valList), EnvID: envQL.ID, ProjID: projectQL.ID, ServID: serviceQL.ID, FileID: fileQL.ID, Key: val.Key, Value: val.Value, Source: val.Source}
						valList = append(valList, valQL)
					}
					(*fileQL).Values = valList
				}
				(*serviceQL).Files = fileList
			}
			(*projectQL).Services = serviceList
		}
		(*envQL).Projects = projectList

		if len(envQL.Providers) == 0 {
			for i := 0; i < len(integrationSessions[env.Name]); i++ {
				integrationSessions[env.Name][i]["EnvID"] = (*envQL).ID
				integrationSessions[env.Name][i]["IntegrationID"] = 0
			}
			for i := 0; i < len(vaultSessions[env.Name]); i++ {
				vaultSessions[env.Name][i]["EnvID"] = (*envQL).ID
				vaultSessions[env.Name][i]["IntegrationID"] = 1
			}

			if integrationSessions[env.Name] != nil {
				(*envQL).Providers = append((*envQL).Providers, Provider{
					EnvID:    (*envQL).ID,
					ID:       0,
					Name:     "Integration Users",
					Sessions: integrationSessions[env.Name],
				})
			}
			if vaultSessions[env.Name] != nil {
				(*envQL).Providers = append((*envQL).Providers, Provider{
					EnvID:    (*envQL).ID,
					ID:       1,
					Name:     "Vault Users",
					Sessions: vaultSessions[env.Name],
				})
			}
		}

	}
	vaultQL := VaultVals{Envs: envList}
	// Convert data to a nested structure
	var ValueObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Value",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"envid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"projid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"servid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},

				"fileid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"key": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						val := params.Source.(Value).ID
						file := params.Source.(Value).FileID

						serv := params.Source.(Value).ServID
						proj := params.Source.(Value).ProjID
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Projects[proj].Services[serv].Files[file].Values[val].Key, nil
					},
				},
				"value": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {

						val := params.Source.(Value).ID
						file := params.Source.(Value).FileID

						serv := params.Source.(Value).ServID
						proj := params.Source.(Value).ProjID
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Projects[proj].Services[serv].Files[file].Values[val].Value, nil
					},
				},
				"source": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {

						val := params.Source.(Value).ID
						file := params.Source.(Value).FileID

						serv := params.Source.(Value).ServID
						proj := params.Source.(Value).ProjID
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Projects[proj].Services[serv].Files[file].Values[val].Source, nil
					},
				},
			},
		})
	var FileObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "File",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"envid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"projid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"servid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},

				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						file := params.Source.(File).ID

						serv := params.Source.(File).ServID
						proj := params.Source.(File).ProjID
						env := params.Source.(File).EnvID
						return vaultQL.Envs[env].Projects[proj].Services[serv].Files[file].Name, nil
					},
				},
				"values": &graphql.Field{
					Type: graphql.NewList(ValueObject),
					Args: graphql.FieldConfigArgument{
						"keyName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
						"sourceName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
						// "valName": &graphql.ArgumentConfig{
						// 	Type: graphql.String,
						// },
					},
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						//get list of values and return
						keyStr, keyOK := params.Args["keyName"].(string)
						// valStr, valOK := params.Args["valName"].(string)
						sourceStr, sourceOK := params.Args["sourceName"].(string)

						file := params.Source.(File).ID

						serv := params.Source.(File).ServID
						proj := params.Source.(File).ProjID
						env := params.Source.(File).EnvID
						values := []Value{}
						if keyOK {
							// Construct a regular expression based on the search
							regex := regexp.MustCompile(`(?i).*` + keyStr + `.*`)
							for i, v := range vaultQL.Envs[env].Projects[proj].Services[serv].Files[file].Values {
								if regex.MatchString(v.Key) {
									values = append(values, vaultQL.Envs[env].Projects[proj].Services[serv].Files[file].Values[i])
								}
							}
						} else {
							values = vaultQL.Envs[env].Projects[proj].Services[serv].Files[file].Values
						}

						if sourceOK {
							filteredValues := []Value{}
							for _, value := range values {
								if value.Source == sourceStr {
									filteredValues = append(filteredValues, value)
								}
							}
							values = filteredValues
						}
						//else if valOK {
						// 	for i, v := range vaultQL.envs[env].services[serv].files[file].values {
						// 		if v.value == valStr {
						// 			return []Value{vaultQL.envs[env].services[serv].files[file].values[i]}, nil
						// 		}
						// 	}
						// 	return vaultQL.envs[env].services[serv].files[file].values, errors.New("valName not found")
						// }
						return values, nil
					},
				},
			},
		})
	var ServiceObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Service",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"envid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"projid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						serv := params.Source.(Service).ID
						proj := params.Source.(Service).ProjID
						env := params.Source.(Service).EnvID
						return vaultQL.Envs[env].Projects[proj].Services[serv].Name, nil
					},
				},
				"files": &graphql.Field{
					Type: graphql.NewList(FileObject),
					Args: graphql.FieldConfigArgument{
						"fileName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						fileStr, isOK := params.Args["fileName"].(string)
						serv := params.Source.(Service).ID
						proj := params.Source.(Service).ProjID
						env := params.Source.(Service).EnvID
						if isOK {
							for i, f := range vaultQL.Envs[env].Projects[proj].Services[serv].Files {
								if f.Name == fileStr {
									return []File{vaultQL.Envs[env].Projects[proj].Services[serv].Files[i]}, nil
								}
							}
							return vaultQL.Envs[env].Projects[proj].Services[serv].Files, errors.New("fileName not found")
						}
						return vaultQL.Envs[env].Projects[proj].Services[serv].Files, nil
					},
				},
			},
		})
	var ProjectObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Project",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"envid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						proj := params.Source.(Project).ID
						env := params.Source.(Project).EnvID
						return vaultQL.Envs[env].Projects[proj].Name, nil
					},
				},
				"services": &graphql.Field{
					Type: graphql.NewList(ServiceObject),
					Args: graphql.FieldConfigArgument{
						"servName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						servStr, isOK := params.Args["servName"].(string)
						proj := params.Source.(Project).ID
						env := params.Source.(Project).EnvID
						if isOK {
							for i, p := range vaultQL.Envs[env].Projects[proj].Services {
								if p.Name == servStr {
									return []Service{vaultQL.Envs[env].Projects[proj].Services[i]}, nil
								}
							}
							return vaultQL.Envs[env].Projects[proj].Services, errors.New("servName not found")
						}
						return vaultQL.Envs[env].Projects[proj].Services, nil
					},
				},
			},
		})

	var SessionObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Session",
			Fields: graphql.Fields{
				"envid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"provvid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"User": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						eID := params.Source.(map[string]interface{})["EnvID"].(int)
						pID := params.Source.(map[string]interface{})["IntegrationID"].(int)
						sID := params.Source.(map[string]interface{})["ID"].(int)
						return vaultQL.Envs[eID].Providers[pID].Sessions[sID]["User"].(string), nil
					},
				},
				"LastLogIn": &graphql.Field{
					Type: graphql.NewNonNull(graphql.Int),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						eID := params.Source.(map[string]interface{})["EnvID"].(int)
						pID := params.Source.(map[string]interface{})["IntegrationID"].(int)
						sID := params.Source.(map[string]interface{})["ID"].(int)
						return vaultQL.Envs[eID].Providers[pID].Sessions[sID]["LastLogIn"].(int64), nil
					},
				},
			},
		},
	)

	var ProviderObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Provider",
			Fields: graphql.Fields{
				"envid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						eid := params.Source.(Provider).EnvID
						pid := params.Source.(Provider).ID
						return vaultQL.Envs[eid].Providers[pid].Name, nil
					},
				},
				"sessions": &graphql.Field{
					Type: graphql.NewList(SessionObject),
					Args: graphql.FieldConfigArgument{
						"userName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						eid := params.Source.(Provider).EnvID
						pid := params.Source.(Provider).ID

						if userName, ok := params.Args["userName"].(string); ok {
							regex := regexp.MustCompile(`(?i).*` + userName + `.*`)
							sessions := []map[string]interface{}{}
							for _, s := range vaultQL.Envs[eid].Providers[pid].Sessions {
								if regex.MatchString(s["User"].(string)) {
									sessions = append(sessions, s)
								}
							}
							return sessions, nil
						}

						return vaultQL.Envs[eid].Providers[pid].Sessions, nil
					},
				},
			},
		},
	)

	var EnvObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Env",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						env := params.Source.(Env).ID
						return vaultQL.Envs[env].Name, nil
					},
				},
				"projects": &graphql.Field{
					Type: graphql.NewList(ProjectObject),
					Args: graphql.FieldConfigArgument{
						"projName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},

					Resolve: func(params graphql.ResolveParams) (interface{}, error) {

						env := params.Source.(Env).ID
						if projStr, ok := params.Args["projName"].(string); ok {
							for i, s := range vaultQL.Envs[env].Projects {
								if s.Name == projStr {
									return []Project{vaultQL.Envs[env].Projects[i]}, nil
								}
							}
							return vaultQL.Envs[env].Projects, errors.New("projName not found")
						}
						return vaultQL.Envs[env].Projects, nil

					},
				},
				"providers": &graphql.Field{
					Type: graphql.NewList(ProviderObject),
					Args: graphql.FieldConfigArgument{
						"provName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						eid := params.Source.(Env).ID
						if provName, ok := params.Args["provName"].(string); ok {
							for _, p := range vaultQL.Envs[eid].Providers {
								if p.Name == provName {
									return []Provider{p}, nil
								}
							}
							return vaultQL.Envs[eid].Providers, errors.New("provName not found")
						}
						if len(vaultQL.Envs[eid].Providers) == 0 {
							return vaultQL.Envs[eid].Providers, errors.New("no providers under environnment")
						}
						return vaultQL.Envs[eid].Providers, nil
					},
				},
			},
		})
	var VaultValObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "VaultVals",

			Fields: graphql.Fields{
				"envs": &graphql.Field{
					Type: graphql.NewList(EnvObject),
					Args: graphql.FieldConfigArgument{
						"envName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						envs := []Env{}
						for _, e := range vaultQL.Envs {
							if e.Name == "dev" || e.Name == "QA" || e.Name == "RQA" || e.Name == "auto" || e.Name == "performance" || e.Name == "itdev" || e.Name == "servicepack" || e.Name == "staging" {
								envs = append(envs, e)
							} else if e.Name == "local/"+params.Context.Value("user").(string) {
								nameBlocks := strings.Split(params.Context.Value("user").(string), "/")
								e.Name = "local-" + nameBlocks[0]
								envs = append(envs, e)
							}
						}

						if envStr, isOK := params.Args["envName"].(string); isOK {
							for i, e := range envs {
								if e.Name == envStr {
									return []Env{envs[i]}, nil
								}
							}
							return envs, fmt.Errorf("envName not found: %s", envStr)
						}
						return envs, nil

					},
				},
			},
		})
	s.GQLSchema, _ = graphql.NewSchema(graphql.SchemaConfig{
		Query: VaultValObject,
	})

}
