package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	pb "github.com/trimble-oss/tierceron/trcweb/rpc/apinator"

	"github.com/graphql-go/graphql"
)

type VaultVals struct {
	ID   string `json:"id"`
	Envs []Env  `json:"envs"`
}
type Env struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Services []Service `json:"services"`
}
type Service struct {
	EnvID int    `json:"envID"`
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Files []File `json:"files"`
}
type File struct {
	EnvID  int     `json:"envID"`
	ServID int     `json:"servID"`
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Values []Value `json:"values"`
}
type Value struct {
	EnvID  int    `json:"envID"`
	ServID int    `json:"servID"`
	FileID int    `json:"fileID"`
	ID     int    `json:"id"`
	Key    string `json:"name"`
	Value  string `json:"value"`
}

func main() {
	addrPtr := flag.String("addr", "http://127.0.0.1:8008", "API endpoint for the vault")
	apiClient := pb.NewEnterpriseServiceBrokerProtobufClient(*addrPtr, &http.Client{})

	makeVaultReq := &pb.GetValuesReq{}
	driverConfig := &config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			ExitOnFailure: true,
		},
	}

	vault, err := apiClient.GetValues(context.Background(), makeVaultReq)
	eUtils.CheckError(driverConfig.CoreConfig, err, true)

	driverConfig.CoreConfig.ExitOnFailure = false

	envList := []Env{}
	for i, env := range vault.Envs {
		serviceList := []Service{}
		for _, project := range env.Projects {
			for k, service := range project.Services {
				fileList := []File{}
				for l, file := range service.Files {
					valList := []Value{}
					for m, val := range file.Values {
						valQL := Value{ID: m, EnvID: i, ServID: k, FileID: l, Key: val.Key, Value: val.Value}
						valList = append(valList, valQL)
					}
					fileQL := File{ID: l, EnvID: i, ServID: k, Name: file.Name, Values: valList}
					fileList = append(fileList, fileQL)
				}
				serviceQL := Service{ID: k, EnvID: i, Name: service.Name, Files: fileList}
				serviceList = append(serviceList, serviceQL)
			}
		}
		envQL := Env{ID: i, Name: env.Name, Services: serviceList}
		envList = append(envList, envQL)
	}

	vaultQL := VaultVals{Envs: envList}
	ValueObject := graphql.NewObject(
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
					Resolve: func(params graphql.ResolveParams) (any, error) {
						val := params.Source.(Value).ID
						file := params.Source.(Value).FileID
						serv := params.Source.(Value).ServID
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Services[serv].Files[file].Values[val].Key, nil
					},
				},
				"value": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (any, error) {
						val := params.Source.(Value).ID
						file := params.Source.(Value).FileID
						serv := params.Source.(Value).ServID
						env := params.Source.(Value).EnvID
						return vaultQL.Envs[env].Services[serv].Files[file].Values[val].Value, nil
					},
				},
			},
		})
	FileObject := graphql.NewObject(
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
					Resolve: func(params graphql.ResolveParams) (any, error) {
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
						// "valName": &graphql.ArgumentConfig{
						// 	Type: graphql.String,
						// },
					},
					Resolve: func(params graphql.ResolveParams) (any, error) {
						// get list of values and return
						keyStr, keyOK := params.Args["keyName"].(string)
						// valStr, valOK := params.Args["valName"].(string)

						file := params.Source.(File).ID
						serv := params.Source.(File).ServID
						env := params.Source.(File).EnvID
						if keyOK {
							for i, v := range vaultQL.Envs[env].Services[serv].Files[file].Values {
								if v.Key == keyStr {
									return []Value{vaultQL.Envs[env].Services[serv].Files[file].Values[i]}, nil
								}
							}
							return []Value{}, errors.New("keyName not found")
						} // else if valOK {
						// 	for i, v := range vaultQL.envs[env].services[serv].files[file].values {
						// 		if v.value == valStr {
						// 			return []Value{vaultQL.envs[env].services[serv].files[file].values[i]}, nil
						// 		}
						// 	}
						// 	return vaultQL.envs[env].services[serv].files[file].values, errors.New("valName not found")
						// }
						return vaultQL.Envs[env].Services[serv].Files[file].Values, nil
						// return nil, nil
					},
				},
			},
		})
	ServiceObject := graphql.NewObject(
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
					Resolve: func(params graphql.ResolveParams) (any, error) {
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
					Resolve: func(params graphql.ResolveParams) (any, error) {
						fileStr, isOK := params.Args["fileName"].(string)
						serv := params.Source.(Service).ID
						env := params.Source.(Service).EnvID
						if isOK {
							for i, f := range vaultQL.Envs[env].Services[serv].Files {
								if f.Name == fileStr {
									return []File{vaultQL.Envs[env].Services[serv].Files[i]}, nil
								}
							}
							return []File{}, errors.New("fileName not found")
						}
						return vaultQL.Envs[env].Services[serv].Files, nil
					},
				},
			},
		})
	EnvObject := graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Env",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
				},
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (any, error) {
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

					Resolve: func(params graphql.ResolveParams) (any, error) {
						servStr, isOK := params.Args["servName"].(string)
						env := params.Source.(Env).ID
						if isOK {
							for i, s := range vaultQL.Envs[env].Services {
								if s.Name == servStr {
									return []Service{vaultQL.Envs[env].Services[i]}, nil
								}
							}
							return []Service{}, errors.New("servName not found")
						}
						return vaultQL.Envs[env].Services, nil
					},
				},
			},
		})
	VaultValObject := graphql.NewObject(
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
					Resolve: func(params graphql.ResolveParams) (any, error) {
						envStr, isOK := params.Args["envName"].(string)
						if isOK {
							for i, e := range vaultQL.Envs {
								if e.Name == envStr {
									return []Env{vaultQL.Envs[i]}, nil
								}
							}
							return []Env{}, errors.New("envName not found")
						}
						return vaultQL.Envs, nil
					},
				},
			},
		})

	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		schema, _ := graphql.NewSchema(graphql.SchemaConfig{
			Query: VaultValObject,
		})
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: r.URL.Query().Get("query"),
		})
		json.NewEncoder(w).Encode(result)
	})

	fmt.Fprintln(os.Stderr, "Server is running on port 8090")
	fmt.Fprintln(os.Stderr, "Test with: curl -g 'http://localhost:8090/graphql?query={envs{services{files{values{key,value}}}}}'")
	http.ListenAndServe(":8090", nil)
}
