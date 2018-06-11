package seeder

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	"gopkg.in/yaml.v2"
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

var logFile *os.File

// SeedVault seeds the vault with seed files in the given directory
func SeedVault(dir string, addr string, token string, env string, f *os.File) {

	logFile = f
	log.SetOutput(logFile)
	log.SetPrefix("Seeder:   ")
	log.Printf("Seeding vault from seeds in: %s\n", dir)

	files, err := ioutil.ReadDir(dir)
	utils.LogError(err, logFile)

	// Iterate through all services
	for _, file := range files {
		if file.IsDir() {
			// Step over directories
			continue
		}
		// Get and check file extension (last substring after .)
		ext := filepath.Ext(file.Name())
		if ext == ".yaml" || ext == ".yml" { // Only read YAML config files
			log.Printf("\tFound seed file: %s\n", file.Name())
			path := dir + "/" + file.Name()
			seedVaultFromFile(path, addr, token, env)
		}

	}

}

func seedVaultFromFile(filepath string, vaultAddr string, token string, env string) {
	rawFile, err := ioutil.ReadFile(filepath)
	// Open file
	utils.LogError(err, logFile)

	// Unmarshal
	var rawYaml interface{}
	err = yaml.Unmarshal(rawFile, &rawYaml)
	utils.LogError(err, logFile)
	seed, ok := rawYaml.(map[interface{}]interface{})
	if ok == false {
		log.Fatal("Count not extract seed from @s. Possibly a formatting issue", filepath)
	}

	mapStack := []seedCollection{seedCollection{"", seed}} // Begin with root of yaml file
	writeStack := make([]writeCollection, 0)               // List of all values to write to the vault with p

	// While the stack is not empty
	for len(mapStack) > 0 {
		current := mapStack[0]
		mapStack = mapStack[1:] // Pop the top value
		writeVals := writeCollection{path: current.path, data: map[string]interface{}{}}
		hasLeafNodes := false // Flag to signify this map had values to write

		// Convert nested maps into vault writable data
		for k, v := range current.data {
			if v == nil { // Don't write empty valus, Vault does not handle them
				log.Printf("Key with no value will not be written: %s\n", current.path+": "+k.(string))
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
	log.Println("Writing seed values to paths")
	mod, err := kv.NewModifier(token, vaultAddr) // Connect to vault
	utils.LogError(err, logFile)
	mod.Env = env
	for _, entry := range writeStack {
		fmt.Println(entry.path) // Output data being written
		for k, v := range entry.data {
			fmt.Printf("\t%-30s%v\n", k, v)
		}

		// Write data and ouput any errors
		warn, err := mod.Write(entry.path, entry.data)
		utils.LogWarnings(warn, logFile)
		utils.LogError(err, logFile)

		// Update value metrics to reflect credential use
		root := strings.Split(entry.path, "/")[0]
		if root == "templates" {
			for _, v := range entry.data {
				if reflect.TypeOf(v) != reflect.TypeOf([]interface{}{}) {
					continue
				}
				if templateKey, ok := v.([]interface{}); ok {
					metricsKey := templateKey[0].(string) + "." + templateKey[1].(string)
					mod.AdjustValue("value-metrics/credentials", metricsKey, 1)
				}
			}
		}
	}
}
