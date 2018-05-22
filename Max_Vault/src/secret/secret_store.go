package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/vault/api"
)

// Access token for the vault server and api address
const RootToken string = "5c567766-160f-bde8-0816-3d5418201a63"
const VaultAddr string = "http://127.0.0.1:8200"

func main() {

	// Create client for server and add token
	client, err := api.NewClient(&api.Config{
		Address: VaultAddr,
	})
	if err != nil {
		fmt.Printf(err.Error() + "\n")
		os.Exit(1)
	}
	client.SetToken(RootToken)
	fmt.Println("Successfully created client")

	// Get logical for API work
	logical := client.Logical()
	secretData := map[string]interface{}{"myKey1": "myVal1"}
	secret, err := logical.Write("secret/goSecret", secretData)

	if err != nil {
		fmt.Println("Failed to write to the vault: \n", err)
		fmt.Println(secret.Warnings)
		for k, v := range secret.Data {
			fmt.Printf("%s : %s\n", k, v)
		}
		os.Exit(1)
	}

	// Write data written to server
	fmt.Println("Wrote secret: ")
	for k, v := range secretData {
		fmt.Printf("%s : %s\n", k, v)
	}
	fmt.Println("To the vault")

	// Try to read the previously written data and print
	secret, err = logical.Read("secret/goSecret")
	if err != nil {
		fmt.Println("Failed to read from the vault: \n", err)
		os.Exit(1)
	}
	fmt.Printf("secret: \n")
	for k, v := range secret.Data {
		fmt.Printf("%s : %s\n", k, v)
	}
}
