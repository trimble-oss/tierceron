package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"

	eUtils "github.com/trimble-oss/tierceron/utils"
	pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"

	"github.com/graphql-go/graphql"
)

type VaultVals struct {
	ID   string `json:"id"`
	envs []Env  `json:"envs"`
}
type Env struct {
	ID       int       `json:"id"`
	name     string    `json:"name"`
	services []Service `json:"services"`
}
type Service struct {
	envID int    `json: "envID"`
	ID    int    `json:"id"`
	name  string `json:"name"`
	files []File `json:"files"`
}
type File struct {
	envID  int     `json: "envID"`
	servID int     `json: "servID"`
	ID     int     `json:"id"`
	name   string  `json:"name"`
	values []Value `json:"values"`
}
type Value struct {
	envID  int    `json: "envID"`
	servID int    `json: "servID"`
	fileID int    `json: "fileID"`
	ID     int    `json:"id"`
	key    string `json:"name"`
	value  string `json:"value"`
}

func main() {

	addrPtr := flag.String("addr", "http://127.0.0.1:8008", "API endpoint for the vault")
	apiClient := pb.NewEnterpriseServiceBrokerProtobufClient(*addrPtr, &http.Client{})

	makeVaultReq := &pb.GetValuesReq{}
	config := &eUtils.DriverConfig{ExitOnFailure: true}

	vault, err := apiClient.GetValues(context.Background(), makeVaultReq)
	eUtils.CheckError(config, err, true)

	config.ExitOnFailure = false

	envList := []Env{}
	for i, env := range vault.Envs {
		serviceList := []Service{}
		for j, service := range env.Services {
			fileList := []File{}
			for k, file := range service.Files {
				valList := []Value{}
				for l, val := range file.Values {
					valQL := Value{ID: l, envID: i, servID: j, fileID: k, key: val.Key, value: val.Value}
					valList = append(valList, valQL)
				}
				fileQL := File{ID: k, envID: i, servID: j, name: file.Name, values: valList}
				fileList = append(fileList, fileQL)
			}
			serviceQL := Service{ID: j, envID: i, name: service.Name, files: fileList}
			serviceList = append(serviceList, serviceQL)
		}
		envQL := Env{ID: i, name: env.Name, services: serviceList}
		envList = append(envList, envQL)
	}

	vaultQL := VaultVals{envs: envList}
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
						file := params.Source.(Value).fileID
						serv := params.Source.(Value).servID
						env := params.Source.(Value).envID
						return vaultQL.envs[env].services[serv].files[file].values[val].key, nil
					},
				},
				"value": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {

						val := params.Source.(Value).ID
						file := params.Source.(Value).fileID
						serv := params.Source.(Value).servID
						env := params.Source.(Value).envID
						return vaultQL.envs[env].services[serv].files[file].values[val].value, nil
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
						serv := params.Source.(File).servID
						env := params.Source.(File).envID
						return vaultQL.envs[env].services[serv].files[file].name, nil
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
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						//get list of values and return
						keyStr, keyOK := params.Args["keyName"].(string)
						// valStr, valOK := params.Args["valName"].(string)

						file := params.Source.(File).ID
						serv := params.Source.(File).servID
						env := params.Source.(File).envID
						if keyOK {
							for i, v := range vaultQL.envs[env].services[serv].files[file].values {
								if v.key == keyStr {
									return []Value{vaultQL.envs[env].services[serv].files[file].values[i]}, nil
								}
							}
							return []Value{}, errors.New("keyName not found")
						} //else if valOK {
						// 	for i, v := range vaultQL.envs[env].services[serv].files[file].values {
						// 		if v.value == valStr {
						// 			return []Value{vaultQL.envs[env].services[serv].files[file].values[i]}, nil
						// 		}
						// 	}
						// 	return vaultQL.envs[env].services[serv].files[file].values, errors.New("valName not found")
						// }
						return vaultQL.envs[env].services[serv].files[file].values, nil
						//return nil, nil
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
						env := params.Source.(Service).envID
						return vaultQL.envs[env].services[serv].name, nil
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
						env := params.Source.(Service).envID
						if isOK {
							for i, f := range vaultQL.envs[env].services[serv].files {
								if f.name == fileStr {
									return []File{vaultQL.envs[env].services[serv].files[i]}, nil
								}
							}
							return []File{}, errors.New("fileName not found")
						}
						return vaultQL.envs[env].services[serv].files, nil
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
						return vaultQL.envs[env].name, nil
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
							for i, s := range vaultQL.envs[env].services {
								if s.name == servStr {
									return []Service{vaultQL.envs[env].services[i]}, nil
								}
							}
							return []Service{}, errors.New("servName not found")
						}
						return vaultQL.envs[env].services, nil

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
						envStr, isOK := params.Args["envName"].(string)
						if isOK {
							for i, e := range vaultQL.envs {
								if e.name == envStr {
									return []Env{vaultQL.envs[i]}, nil
								}
							}
							return []Env{}, errors.New("envName not found")
						}
						return vaultQL.envs, nil

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

	fmt.Println("Server is running on port 8090")
	fmt.Println("Test with: curl -g 'http://localhost:8090/graphql?query={envs{services{files{values{key,value}}}}}'")
	http.ListenAndServe(":8090", nil)
}
