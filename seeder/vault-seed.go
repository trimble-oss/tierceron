package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"vault-helper/kv"

	"github.com/smallfish/simpleyaml"
)

// Used in the decomposition of the seed
type seedCollection struct {
	path string
	data map[interface{}]interface{}
}

// Used for containing the actual paths and vlues to vault
type writeCollection struct {
	path string
	data map[string]interface{}
}

// Simplifies the error checking process
func checkError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func main() {
	// Flags needed for connecting and seeding
	dirPtr := flag.String("dir", "seeds", "Directory containing seed files for vault")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")

	flag.Parse()
	fmt.Printf("Seeding vault from templates in: %s\n", *dirPtr)
	fmt.Printf("Token: %s\nAddress: %s\n", *tokenPtr, *addrPtr)

	files, err := ioutil.ReadDir(*dirPtr)
	checkError(err)

	// Iterate through all services
	for _, file := range files {
		if file.IsDir() {
			// Step over directories
			continue
		}
		// Get and check file extension (last substring after .)
		ext := func(f string) string {
			chunks := strings.SplitAfter(f, ".")
			return chunks[len(chunks)-1]
		}(file.Name())
		if ext == "yaml" || ext == "yml" { // Only read YAML config files
			fmt.Printf("\nSeeding from YAML file: %s\n", file.Name())
			filepath := *dirPtr + "/" + file.Name()
			seedVaultFromFile(filepath, *addrPtr, *tokenPtr)
		}

	}

}

func seedVaultFromFile(filepath string, vaultAddr string, token string) {
	rawFile, err := ioutil.ReadFile(filepath)
	// Open file
	checkError(err)

	// Unmarshal
	yaml, err := simpleyaml.NewYaml(rawFile)
	seed, _ := yaml.Map()

	mapStack := make([]seedCollection, 0)    // Working stack of nested maps to decompose
	writeStack := make([]writeCollection, 0) // List of all values to write to the vault with p
	for baseK, baseV := range seed {         // Add base maps to stack to avoid adding the no-key root map
		if baseV != nil {
			mapStack = append(mapStack, seedCollection{baseK.(string), baseV.(map[interface{}]interface{})})
		}
	}

	// While the stack is not empty
	for len(mapStack) > 0 {
		current := mapStack[0]
		mapStack = mapStack[1:] // Pop the top value
		writeVals := writeCollection{path: current.path, data: map[string]interface{}{}}
		hasLeafNodes := false // Flag to signify this map had values to write

		// Convert nested maps into vault writable data
		for k, v := range current.data {
			if v == nil { // Don't write empty valus, Vault does not handle them
				fmt.Printf("Key with no value will not be written: %s\n", current.path+": "+k.(string))
			} else if newData, ok := v.(map[interface{}]interface{}); ok { // Decompose into submaps, update path
				decomp := seedCollection{
					path: current.path + "/" + k.(string),
					data: newData}
				mapStack = append([]seedCollection{decomp}, mapStack...)
			} else { // Found a key value pair, add to working writeVal
				writeVals.data[k.(string)] = v
				hasLeafNodes = true
			}
		}
		if hasLeafNodes { // Save all writable values in the current path
			writeStack = append(writeStack, writeVals)
		}
	}

	// Write values to vault
	fmt.Println("Writing seed values to paths")
	mod, err := kv.NewModifier(token, vaultAddr) // Connect to vault
	checkError(err)
	for _, entry := range writeStack {
		fmt.Println(entry.path) // Output data being written
		for k, v := range entry.data {
			fmt.Printf("\t%-30s%v\n", k, v)
		}
		warnings, err := mod.Write(entry.path, entry.data)
		if warnings != nil { // Output any warnings the Vault generates
			fmt.Println(warnings)
		}
		checkError(err)
	}
}
