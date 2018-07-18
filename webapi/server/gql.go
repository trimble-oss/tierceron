package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"

	"bitbucket.org/dexterchaney/whoville/utils"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
	"github.com/graphql-go/graphql"
)

//VaultVals Holds environments, used for GraphQL
type VaultVals struct {
	ID   string `json:"id"`
	Envs []Env  `json:"envs"`
}

//Env represents an environment containing multiple services
type Env struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Services []Service `json:"services"`
}

//Service represents an service that contains multiple files
type Service struct {
	EnvID int    `json:"envID"`
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Files []File `json:"files"`
}

//File represents an individual file containing template values
type File struct {
	EnvID  int     `json:"envID"`
	ServID int     `json:"servID"`
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Values []Value `json:"values"`
}

//Value represents an individual key-value pair with source
type Value struct {
	EnvID  int    `json:"envID"`
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

//GraphQL Accepts a GraphQL query and creates a response
func (s *Server) GraphQL(ctx context.Context, req *pb.GraphQLQuery) (*pb.GraphQLResp, error) {
	rawResult := graphql.Do(graphql.Params{
		Schema:        s.GQLSchema,
		RequestString: req.Query,
	})

	result := &pb.GraphQLResp{}
	resultBytes := bytes.NewBuffer(nil)
	json.NewEncoder(resultBytes).Encode(rawResult)
	json.Unmarshal(resultBytes.Bytes(), result)
	return result, nil
}

//InitGQL Initializes the GQL schema
func (s *Server) InitGQL() {
	makeVaultReq := &pb.GetValuesReq{}

	// Fetch template keys and values
	vault, err := s.GetValues(context.Background(), makeVaultReq)
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		utils.LogWarningsObject([]string{"GraphQL MAY not initialized (values not added)"}, s.Log, false)
		return
	}

	// Fetch secret keys and verification info
	templates, err := s.getTemplateData()
	if err != nil {
		utils.LogErrorObject(err, s.Log, false)
		utils.LogWarningsObject([]string{"GraphQL MAY not initialized (secrets not added)"}, s.Log, false)
		return
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
			envList = append(envList, Env{ID: len(envList), Name: env.Name, Services: []Service{}})
			envQL = &envList[len(envList)-1]
		}

		serviceIndices := envIndices[env.Name].children // Track indices of services in list
		serviceList := append([]Service{}, envQL.Services...)

		// Service
		for _, service := range env.Services {
			var serviceQL *Service
			if serviceIndices[service.Name] != nil { // Get a reference to the existing service
				index := serviceIndices[service.Name].index
				serviceQL = &serviceList[index]
			} else { // Create a new service
				serviceIndices[service.Name] = &visitedNode{index: len(serviceList), children: map[string]*visitedNode{}}
				serviceList = append(serviceList, Service{ID: len(serviceList), EnvID: envQL.ID, Name: service.Name, Files: []File{}})
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
					fileList = append(fileList, File{ID: len(fileList), EnvID: envQL.ID, ServID: serviceQL.ID, Name: file.Name, Values: []Value{}})
					fileQL = &fileList[len(fileList)-1]
				}

				// Values
				valList := fileQL.Values
				for _, val := range file.Values {
					valQL := Value{ID: len(valList), EnvID: envQL.ID, ServID: serviceQL.ID, FileID: fileQL.ID, Key: val.Key, Value: val.Value, Source: val.Source}
					valList = append(valList, valQL)
				}
				(*fileQL).Values = valList
			}
			(*serviceQL).Files = fileList
		}
		(*envQL).Services = serviceList

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
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Services[serv].Files[file].Values[val].Key, nil
					},
				},
				"value": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {

						val := params.Source.(Value).ID
						file := params.Source.(Value).FileID
						serv := params.Source.(Value).ServID
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Services[serv].Files[file].Values[val].Value, nil
					},
				},
				"source": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {

						val := params.Source.(Value).ID
						file := params.Source.(Value).FileID
						serv := params.Source.(Value).ServID
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Services[serv].Files[file].Values[val].Source, nil
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
				"servid": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						file := params.Source.(File).ID
						serv := params.Source.(File).ServID
						env := params.Source.(File).EnvID
						return vaultQL.Envs[env].Services[serv].Files[file].Name, nil
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
						env := params.Source.(File).EnvID
						values := []Value{}
						if keyOK {
							// Construct a regular expression based on the search
							regex := regexp.MustCompile(`(?i).*` + keyStr + `.*`)
							for i, v := range vaultQL.Envs[env].Services[serv].Files[file].Values {
								if regex.MatchString(v.Key) {
									values = append(values, vaultQL.Envs[env].Services[serv].Files[file].Values[i])
								}
							}
						} else {
							values = vaultQL.Envs[env].Services[serv].Files[file].Values
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
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						serv := params.Source.(Service).ID
						env := params.Source.(Service).EnvID
						return vaultQL.Envs[env].Services[serv].Name, nil
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
						env := params.Source.(Service).EnvID
						if isOK {
							for i, f := range vaultQL.Envs[env].Services[serv].Files {
								if f.Name == fileStr {
									return []File{vaultQL.Envs[env].Services[serv].Files[i]}, nil
								}
							}
							return vaultQL.Envs[env].Services[serv].Files, errors.New("fileName not found")
						}
						return vaultQL.Envs[env].Services[serv].Files, nil
					},
				},
			},
		})
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
				"services": &graphql.Field{
					Type: graphql.NewList(ServiceObject),
					Args: graphql.FieldConfigArgument{
						"servName": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},

					Resolve: func(params graphql.ResolveParams) (interface{}, error) {

						servStr, isOK := params.Args["servName"].(string)
						env := params.Source.(Env).ID
						if isOK {
							for i, s := range vaultQL.Envs[env].Services {
								if s.Name == servStr {
									return []Service{vaultQL.Envs[env].Services[i]}, nil
								}
							}
							return vaultQL.Envs[env].Services, errors.New("servName not found")
						}
						return vaultQL.Envs[env].Services, nil

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
						if envStr, isOK := params.Args["envName"].(string); isOK {
							for i, e := range vaultQL.Envs {
								if e.Name == envStr {
									return []Env{vaultQL.Envs[i]}, nil
								}
							}
							return vaultQL.Envs, errors.New("envName not found")
						}
						return vaultQL.Envs, nil

					},
				},
			},
		})
	s.GQLSchema, _ = graphql.NewSchema(graphql.SchemaConfig{
		Query: VaultValObject,
	})

}
