package main

import (
	"flag"
	"fmt"
	"log"
	"vault-helper/kv"
)

// Quick test for the Modifier struct and relevent functions
// Command line args: -t=<auth token> -p=<secret path>
func main() {
	tokenPtr := flag.String("t", "", "auth token for the vault")
	pathPtr := flag.String("p", "secret/default", "vault path to read/write secrets")

	flag.Parse()

	if len(*tokenPtr) == 0 {
		fmt.Println("Token required to access vault (-t=<token>)")
		return
	}

	mod, err := kv.NewModifier(*tokenPtr, "")

	if err != nil {
		log.Fatal(err)
	}

	mod.Path = *pathPtr
	testData := map[string]interface{}{
		"Unseal Key": "2cQLr6jlScYZSCHxuCw0xXPAkOIQCbqVpkzd9kW55rM=",
		"Root Token": "af2dde62-f9a8-5887-960f-f1dd8a70773c",
		"My random":  "A random secret"}

	warnings, err := mod.Write(testData)

	if len(warnings) > 0 {
		// Output any warnings
		fmt.Println("Warnings:")
		fmt.Println(warnings)
		fmt.Println("\n")
	}
	if err != nil {
		log.Fatal(err)
	}

	// Test reading
	Secret, err := mod.Read()

	if len(Secret.Warnings) > 0 {
		// Output any warnings
		fmt.Println("Warnings:")
		fmt.Println(Secret.Warnings)
		fmt.Println("\n")
	}
	if err != nil {
		log.Fatal(err)
	}

	// Output the data retrived
	fmt.Println("Retrieved Secret\n================")
	fmt.Println("LeaseID = " + Secret.LeaseID)
	fmt.Printf("Lease Duration = %d\n", Secret.LeaseDuration)
	fmt.Printf("Renewable = %t\n", Secret.Renewable)
	fmt.Println("\nData <Key,Val>")
	for k, v := range Secret.Data {
		fmt.Printf("%s: %s\n", k, v)
	}
}
