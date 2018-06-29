package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"

	"bitbucket.org/dexterchaney/whoville/utils"
	pb "bitbucket.org/dexterchaney/whoville/webapi/rpc/apinator"
	"github.com/graphql-go/graphql"
)

type VaultVals struct {
	ID   string `json:"_id"`
	envs []Env  `json:"envs"`
}
type Env struct {
	ID       string    `json:"_id"`
	name     string    `json:"name"`
	services []Service `json:"services"`
}
type Service struct {
	ID    string `json:"_id"`
	name  string `json:"name"`
	files []File `json:"files"`
}
type File struct {
	ID     string  `json:"_id"`
	name   string  `json:"name"`
	values []Value `json:"values"`
}
type Value struct {
	ID    string `json:"_id"`
	key   string `json:"name"`
	value string `json:"value"`
}

var ValueObject = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Value",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"key": &graphql.Field{
				Type: graphql.String,
			},
			"value": &graphql.Field{
				Type: graphql.String,
				//return vaultQL.envs[0].services[0].files[0].values[0], nil
			},
		},
	})
var FileObject = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "File",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"values": &graphql.Field{
				Type: graphql.NewList(ValueObject),
			},
		},
	})
var ServiceObject = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Service",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"files": &graphql.Field{
				Type: graphql.NewList(FileObject),
			},
		},
	})
var EnvObject = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Env",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"services": &graphql.Field{
				Type: graphql.NewList(ServiceObject),
			},
		},
	})
var VaultValObject = graphql.NewObject(
	graphql.ObjectConfig{

		Name: "VaultVals",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},

			"envs": &graphql.Field{
				Type: graphql.NewList(EnvObject),
			},
		},
	})

var fakeVault VaultVals = VaultVals{envs: []Env{Env{ID: "1", name: "fakeEnv", services: []Service{Service{ID: "1", name: "fakeService", files: []File{File{ID: "1", name: "fakeFile", values: []Value{Value{ID: "1", key: "fakeKey", value: "fakeValue"}}}}}}}}}

// var schema, _ = graphql.NewSchema(
// 	graphql.SchemaConfig{
// 		Query: VaultValObject,
// 	},
// )
func Filter(envs []Env, f func(Env) bool) []Env {
	fmt.Println("filtering")
	vsf := make([]Env, 0)
	for _, v := range envs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	if len(vsf) == 0 {
		fmt.Println("filtered everything out")
	}
	return vsf
}

func main() {

	addrPtr := flag.String("addr", "http://127.0.0.1:8008", "API endpoint for the vault")
	apiClient := pb.NewEnterpriseServiceBrokerProtobufClient(*addrPtr, &http.Client{})

	makeVaultReq := &pb.GetValuesReq{}

	vault, err := apiClient.GetValues(context.Background(), makeVaultReq)
	utils.CheckError(err)

	envList := []Env{}
	fmt.Printf("Vault: \n")
	for i, env := range vault.Envs {
		serviceList := []Service{}
		fmt.Printf("Env: %s\n", env.Name)
		for j, service := range env.Services {
			fileList := []File{}
			fmt.Printf("\tService: %s\n", service.Name)
			for k, file := range service.Files {
				valList := []Value{}
				fmt.Printf("\t\tFile: %s\n", file.Name)
				for l, val := range file.Values {
					fmt.Printf("\t\t\tkey: %s\tvalue: %s\n", val.Key, val.Value)
					valQL := Value{ID: string(l), key: val.Key, value: val.Value}
					valList = append(valList, valQL)
				}
				fileQL := File{ID: string(k), name: file.Name, values: valList}
				fileList = append(fileList, fileQL)
			}
			serviceQL := Service{ID: string(j), name: service.Name, files: fileList}
			serviceList = append(serviceList, serviceQL)
		}
		envQL := Env{ID: string(i), name: env.Name, services: serviceList}
		envList = append(envList, envQL)
	}
	vaultQL := VaultVals{envs: envList}
	fmt.Println(vaultQL)
	var ValueObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Values",
			Fields: graphql.Fields{
				"key": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						return vaultQL.envs[0].services[0].files[0].values[0].key, nil
					},
				},
				"value": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						return vaultQL.envs[0].services[0].files[0].values[0].value, nil
					},
					//return vaultQL.envs[0].services[0].files[0].values[0], nil
				},
			},
		})
	var FileObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "File",
			Fields: graphql.Fields{
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						return vaultQL.envs[0].services[0].files[0].name, nil
					},
				},
				"values": &graphql.Field{
					Type: graphql.NewList(ValueObject),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						//get list of values and return
						// ValueList := []Value{}
						// return ValueList, nil
						return vaultQL.envs[0].services[0].files[0].values, nil
					},
				},
			},
		})
	var ServiceObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Service",
			Fields: graphql.Fields{
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						return vaultQL.envs[0].services[0].name, nil
					},
				},
				"files": &graphql.Field{
					Type: graphql.NewList(FileObject),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						//get list of files and return
						// FileList := []File{}
						// return FileList, nil
						return vaultQL.envs[0].services[0].files, nil
					},
				},
			},
		})
	var EnvObject = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Env",
			Fields: graphql.Fields{
				"name": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						return vaultQL.envs[0].name, nil
					},
				},
				"services": &graphql.Field{
					Type: graphql.NewList(ServiceObject),
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						//get list of services and return
						//serviceList := []Service{}
						// serviceName := params.Args["name"].(string)
						// for _, service := range serviceList {
						// 	if service.name == serviceName {
						// 		return service, nil
						// 	}
						// }
						//fmt.Println(fakeVault.envs[0].services)
						return vaultQL.envs[0].services, nil
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
					Resolve: func(params graphql.ResolveParams) (interface{}, error) {
						fmt.Println("resolving")
						//fmt.Println(vaultQL.envs)
						//add logic to retrieve list of envs
						// envName := params.Args["name"].(string)
						// filtered := Filter(vaultQL.envs, func(v Env) bool {
						// 	fmt.Println("this one")
						// 	return strings.Contains(v.name, envName)
						// })
						fmt.Println(fakeVault.envs)
						return vaultQL.envs, nil
						//return envList, nil
					},
				},
			},
		})

	//importJSONData()

	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("executing query")
		schema, _ := graphql.NewSchema(graphql.SchemaConfig{
			Query: VaultValObject,
		})
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: r.URL.Query().Get("query"),
		})
		json.NewEncoder(w).Encode(result)
	})

	fmt.Println("Now server is running on port 8080")
	fmt.Println("Test with Get      : curl -g 'http://localhost:8080/graphql?query={envs{services{files{values{key}}}}}'")
	http.ListenAndServe(":8080", nil)
}
