package initlib

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/validator"
	"bitbucket.org/dexterchaney/whoville/vaulthelper/kv"
	"gopkg.in/yaml.v2"
)

// Used in the decomposition of the seed
type seedCollection struct {
	path string
	data map[interface{}]interface{}
}

// Used for containing the actual paths and values to vault
type writeCollection struct {
	path string
	data map[string]interface{}
}

// SeedVault seeds the vault with seed files in the given directory
func SeedVault(dir string, addr string, token string, env string, logger *log.Logger, service string) {
	logger.SetPrefix("[SEED]")
	logger.Printf("Seeding vault from seeds in: %s\n", dir)

	files, err := ioutil.ReadDir(dir)
	utils.LogErrorObject(err, logger, true)

	for _, file := range files {
		if file.Name() == env || (strings.HasPrefix(env, "local") && file.Name() == "local") {
			logger.Println("\tStepping into: " + file.Name())

			filesSteppedInto, err := ioutil.ReadDir(dir + "/" + env)
			utils.LogErrorObject(err, logger, true)

			if len(filesSteppedInto) > 1 {
				utils.CheckWarning(fmt.Sprintf("Multiple potentially conflicting configuration files found for evironment: %s", file.Name()), true)
			}
			for _, fileSteppedInto := range filesSteppedInto {
				ext := filepath.Ext(fileSteppedInto.Name())
				if ext == ".yaml" || ext == ".yml" { // Only read YAML config files
					logger.Println("\t\t" + fileSteppedInto.Name())
					logger.Printf("\tFound seed file: %s\n", fileSteppedInto.Name())
					path := dir + "/" + env + "/" + fileSteppedInto.Name()
					logger.Println("\tSeeding vault with: " + fileSteppedInto.Name())

					SeedVaultFromFile(path, addr, token, env, logger, service)
				}
			}
		}
	}
}

//SeedVaultFromFile takes a file path and seeds the vault with the seeds found in an individual file
func SeedVaultFromFile(filepath string, vaultAddr string, token string, env string, logger *log.Logger, service string) {
	rawFile, err := ioutil.ReadFile(filepath)
	// Open file
	utils.LogErrorObject(err, logger, true)
	SeedVaultFromData(rawFile, vaultAddr, token, env, logger, service)
}

//SeedVaultFromData takes file bytes and seeds the vault with contained data
func SeedVaultFromData(fData []byte, vaultAddr string, token string, env string, logger *log.Logger, service string) {
	logger.SetPrefix("[SEED]")
	logger.Println("=========New File==========")
	var verificationData map[interface{}]interface{} // Create a reference for verification. Can't run until other secrets written
	// Unmarshal
	var rawYaml interface{}
	if bytes.Contains(fData, []byte("<Enter Secret Here>")) {
		fmt.Println("Incomplete configuration of seed data.  Found default secret data: '<Enter Secret Here>'.  Refusing to continue.")
		os.Exit(1)
	}
	err := yaml.Unmarshal(fData, &rawYaml)
	utils.LogErrorObject(err, logger, true)
	seed, ok := rawYaml.(map[interface{}]interface{})
	if ok == false {
		fmt.Println("Invalid yaml file.  Refusing to continue.")
		os.Exit(1)
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
	logger.Println("Seeding configuration data for the following templates:")
	logger.Println("Please verify that these templates exist in each service")

	mod, err := kv.NewModifier(token, vaultAddr, env, nil) // Connect to vault
	utils.LogErrorObject(err, logger, true)
	mod.Env = env
	for _, entry := range writeStack {
		// Output data being written
		// Write data and ouput any errors
		if strings.HasPrefix(entry.path, "values/") {
			if certPathData, certPathOk := entry.data["certSourcePath"]; certPathOk {
				certPath := fmt.Sprintf("%s", certPathData)
				fmt.Println("Inspecting certificate: " + certPath + ".")

				if strings.Contains(certPath, "ENV") {
					if len(env) >= 5 && (env)[:5] == "local" {
						envParts := strings.SplitN(env, "/", 3)
						certPath = strings.Replace(certPath, "ENV", envParts[1], 1)
					} else {
						certPath = strings.Replace(certPath, "ENV", env, 1)
					}
				}
				certPath = "vault_seeds/" + certPath
				cert, err := ioutil.ReadFile(certPath)
				utils.LogErrorObject(err, logger, false)
				if err == nil {
					//if pfx file size greater than 25 KB, print warning
					if len(cert) > 32000 {
						fmt.Println("Unreasonable size for pfx file. Not written to vault")
						continue
					}

					isValidCert := false
					var certValidationErr error
					if strings.HasSuffix(certPath, ".pfx") {
						fmt.Println("Inspecting pfx: " + certPath + ".")
						isValidCert, certValidationErr = validator.IsPfxRfc7292(cert)
					} else if strings.HasSuffix(certPath, ".cer") {
						fmt.Println("Inspecting cer: " + certPath + ".")
						cert, certValidationErr := x509.ParseCertificate(cert)
						if certValidationErr == nil {
							isValidCert = true
						} else {
							fmt.Println("failed to verify certificate: " + certValidationErr.Error())
						}
						var certHost string
						if certHostData, certHostOk := entry.data["certHost"]; certHostOk {
							certHost = fmt.Sprintf("%s", certHostData)
						} else {
							fmt.Println("Missing certHost, cannot validate cert.  Not written to vault")
							continue
						}
						opts := x509.VerifyOptions{
							DNSName: certHost,
						}

						if _, err := cert.Verify(opts); err != nil {
							if _, isUnknownAuthority := err.(x509.UnknownAuthorityError); !isUnknownAuthority {
								fmt.Println("failed to verify certificate: " + err.Error())
								continue
							}
						}

					}
					if isValidCert {
						fmt.Println("Certificate passed validation: " + certPath + ".")
						certBase64 := base64.StdEncoding.EncodeToString(cert)
						if _, ok := entry.data["certData"]; ok {
							// insecure value entry.
							entry.data["certData"] = certBase64
							fmt.Println("Public cert updated: " + certPath + ".")
						} else {
							entryPathParts := strings.Split(entry.path, "/")
							if len(entryPathParts) == 2 {
								secretPath := "super-secrets/" + entryPathParts[1]
								done := false
								// Look up in private entry.
								for _, secretEntry := range writeStack {
									if secretPath == secretEntry.path {
										if _, ok := secretEntry.data["certData"]; ok {
											secretEntry.data["certData"] = certBase64
											WriteData(secretEntry.path, secretEntry.data, mod, logger)
											WriteData(entry.path, entry.data, mod, logger)
											done = true
											break
										}
									}
								}
								fmt.Println("Cert loaded from: " + certPath + ".")

								if done {
									continue
								}
							}
						}
					} else {
						fmt.Println("Cert validation failure.  Cert will not be loaded.", certValidationErr)
						delete(entry.data, "certData")
						delete(entry.data, "certHost")
						delete(entry.data, "certSourcePath")
						delete(entry.data, "certDestPath")
					}
				} else {
					fmt.Println("Missing expected cert at: " + certPath + ".  Cert will not be loaded.")
				}
			}
		}

		if service != "" {
			if strings.HasSuffix(entry.path, service) || strings.Contains(entry.path, "Common") {
				WriteData(entry.path, entry.data, mod, logger)
			}
		} else {

			WriteData(entry.path, entry.data, mod, logger)
		}
	}

	// Run verification after seeds have been written
	warn, err := verify(mod, verificationData, logger)
	utils.LogErrorObject(err, logger, false)
	utils.LogWarningsObject(warn, logger, false)
	fmt.Println("Initialization complete.")
}

//WriteData takes entry path and date from each iteration of writeStack in SeedVaultFromData and writes to vault
func WriteData(path string, data map[string]interface{}, mod *kv.Modifier, logger *log.Logger) {
	warn, err := mod.Write(path, data)

	utils.LogWarningsObject(warn, logger, false)
	utils.LogErrorObject(err, logger, false)
	// Update value metrics to reflect credential use
	root := strings.Split(path, "/")[0]
	if root == "templates" {
		//Printing out path of each entry so that users can verify that folder structure in seed files are correct

		logger.Println("vault_" + path + ".*.tmpl")
		for _, v := range data {
			if templateKey, ok := v.([]interface{}); ok {
				metricsKey := templateKey[0].(string) + "." + templateKey[1].(string)
				mod.AdjustValue("value-metrics/credentials", metricsKey, 1)
			}
		}
	}

}
