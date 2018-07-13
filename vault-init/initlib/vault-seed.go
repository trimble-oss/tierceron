package initlib

import (
	"io/ioutil"
	"log"
	"path/filepath"
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

// SeedVault seeds the vault with seed files in the given directory
func SeedVault(dir string, addr string, token string, env string, logger *log.Logger, certPath string) {
	logger.SetPrefix("[SEED]")
	logger.Printf("Seeding vault from seeds in: %s\n", dir)

	files, err := ioutil.ReadDir(dir)
	utils.LogErrorObject(err, logger, true)

	// Iterate through all services
	for _, file := range files {
		if file.IsDir() {
			// Step over directories
			continue
		}
		// Get and check file extension (last substring after .)
		ext := filepath.Ext(file.Name())
		if ext == ".yaml" || ext == ".yml" { // Only read YAML config files
			logger.Printf("\tFound seed file: %s\n", file.Name())
			path := dir + "/" + file.Name()
			SeedVaultFromFile(path, addr, token, env, certPath, logger)
		}

	}

}

//SeedVaultFromFile takes a file path and seeds the vault with the seeds found in an individual file
func SeedVaultFromFile(filepath string, vaultAddr string, token string, env string, certPath string, logger *log.Logger) {
	rawFile, err := ioutil.ReadFile(filepath)
	// Open file
	utils.LogErrorObject(err, logger, true)
	SeedVaultFromData(rawFile, vaultAddr, token, env, certPath, logger)
}

//SeedVaultFromData takes file bytes and seeds the vault with contained data
func SeedVaultFromData(fData []byte, vaultAddr string, token string, env string, certPath string, logger *log.Logger) {
	logger.SetPrefix("[SEED]")
	logger.Println("=========New File==========")
	var verificationData map[interface{}]interface{} // Create a reference for verification. Can't run until other secrets written
	// Unmarshal
	var rawYaml interface{}
	err := yaml.Unmarshal(fData, &rawYaml)
	utils.LogErrorObject(err, logger, true)
	seed, ok := rawYaml.(map[interface{}]interface{})
	if ok == false {
		logger.Fatal("Count not extract seed. Possibly a formatting issue")
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
				logger.Printf("Key with no value will not be written: %s\n", current.path+": "+k.(string))
			} else if current.path == "" && k.(string) == "verification" { // Found verification on top level, store for later
				verificationData = v.(map[interface{}]interface{})
			} else if newData, ok := v.(map[interface{}]interface{}); ok { // Decompose into submaps, update path
				decomp := seedCollection{
					data: newData}
				if len(current.path) == 0 {
					decomp.path = k.(string)
				} else {
					decomp.path = current.path + "/" + k.(string)
				}
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
	logger.Println("Writing seed values to paths")
	mod, err := kv.NewModifier(token, vaultAddr, certPath) // Connect to vault
	utils.LogErrorObject(err, logger, true)
	mod.Env = env
	for _, entry := range writeStack {
		// Output data being written
		// Write data and ouput any errors
		warn, err := mod.Write(entry.path, entry.data)
		utils.LogWarningsObject(warn, logger, false)
		utils.LogErrorObject(err, logger, false)
		// Update value metrics to reflect credential use
		root := strings.Split(entry.path, "/")[0]
		if root == "templates" {
			for _, v := range entry.data {
				if templateKey, ok := v.([]interface{}); ok {
					metricsKey := templateKey[0].(string) + "." + templateKey[1].(string)
					mod.AdjustValue("value-metrics/credentials", metricsKey, 1)
				}
			}
		}
	}

	// Run verification after seeds have been written
	warn, err := verify(mod, verificationData, logger)
	utils.LogErrorObject(err, logger, false)
	utils.LogWarningsObject(warn, logger, false)
}
