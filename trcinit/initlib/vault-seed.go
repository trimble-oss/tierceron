package initlib

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"tierceron/trcx/xutil"
	"tierceron/utils"
	"tierceron/validator"
	"tierceron/vaulthelper/kv"

	eUtils "tierceron/utils"

	configcore "VaultConfig.Bootstrap/configcore"
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

var templateWritten map[string]bool

// SeedVault seeds the vault with seed files in the given directory -> only init uses this
func SeedVault(insecure bool,
	dir string,
	addr string,
	token string,
	env string,
	subSectionSlice []string,
	logger *log.Logger,
	service string,
	uploadCert bool) error {

	logger.SetPrefix("[SEED]")
	logger.Printf("Seeding vault from seeds in: %s\n", dir)

	files, err := ioutil.ReadDir(dir)

	templateWritten = make(map[string]bool)
	var config *utils.DriverConfig
	if len(files) == 1 && files[0].Name() == "certs" && uploadCert {
		// Cert rotation support without templates
		logger.Printf("No templates available, Common service requested.: %s\n", dir)

		var templatePaths = configcore.GetSupportedTemplates()
		regions := []string{}

		if strings.HasPrefix(env, "staging") || strings.HasPrefix(env, "prod") || strings.HasPrefix(env, "dev") {
			regions = utils.GetSupportedProdRegions()
		}

		config = &utils.DriverConfig{
			Insecure:       insecure,
			Token:          token,
			VaultAddress:   addr,
			Env:            env,
			Regions:        regions,
			SecretMode:     true, //  "Only override secret values in templates?"
			ServicesWanted: []string{service},
			StartDir:       append([]string{}, ""),
			EndDir:         "",
			WantCerts:      false,
			GenAuth:        false,
			Log:            logger,
		}

		_, _, seedData, errGenerateSeeds := xutil.GenerateSeedsFromVaultRaw(config, true, templatePaths)
		if errGenerateSeeds != nil {
			return eUtils.LogErrorAndSafeExit(config, errGenerateSeeds, -1)
		}

		seedData = strings.ReplaceAll(seedData, "<Enter Secret Here>", "")

		SeedVaultFromData(config, "", []byte(seedData), service, true)
		return nil
	} else {
		config = &utils.DriverConfig{
			Insecure:       insecure,
			Token:          token,
			VaultAddress:   addr,
			Env:            env,
			SecretMode:     true, //  "Only override secret values in templates?"
			ServicesWanted: []string{service},
			StartDir:       append([]string{}, ""),
			EndDir:         "",
			WantCerts:      false,
			GenAuth:        false,
			Log:            logger,
		}

	}
	utils.LogErrorObject(config, err, true)

	_, suffix, indexedEnvNot, _ := kv.PreCheckEnvironment(env)

	seeded := false
	starEnv := false
	if strings.Contains(env, "*") {
		starEnv = true
		env = strings.Split(env, "*")[0]
	}
	for _, envDir := range files {
		if strings.HasPrefix(env, envDir.Name()) || (strings.HasPrefix(env, "local") && envDir.Name() == "local") {
			logger.Println("\tStepping into: " + envDir.Name())
			var filesSteppedInto []fs.FileInfo
			if indexedEnvNot {
				filesSteppedInto, err = ioutil.ReadDir(dir + "/" + envDir.Name() + "/" + suffix)
			} else {
				filesSteppedInto, err = ioutil.ReadDir(dir + "/" + envDir.Name())
			}
			utils.LogErrorObject(config, err, true)

			conflictingFile := false
			for _, fileSteppedInto := range filesSteppedInto {
				if !strings.HasPrefix(fileSteppedInto.Name(), env) {
					if strings.Contains(env, ".") {
						secondCheck := strings.Split(env, ".")[0]
						if !strings.HasPrefix(fileSteppedInto.Name(), secondCheck) {
							conflictingFile = true
							logger.Printf("Found conflicting env seed file: %s \n", fileSteppedInto.Name())
						}
					}
				}
			}
			if len(filesSteppedInto) > 1 && conflictingFile {
				utils.CheckWarning(config, fmt.Sprintf("Multiple potentially conflicting configuration files found for environment: %s", envDir.Name()), true)
			}

			normalEnv := false
			if !starEnv && !strings.Contains(env, ".") {
				normalEnv = true
			}

			for _, fileSteppedInto := range filesSteppedInto {
				if fileSteppedInto.Name() == "Index" || fileSteppedInto.Name() == "Restricted" {
					projectDirectories, err := ioutil.ReadDir(dir + "/" + envDir.Name() + "/" + fileSteppedInto.Name())
					if err != nil {
						logger.Printf("Couldn't read into: %s \n", fileSteppedInto.Name())
					}
					// Iterate of projects...
					for _, projectDirectory := range projectDirectories {
						if len(subSectionSlice) > 0 {
							acceptProject := false
							for _, index := range subSectionSlice {
								if index == projectDirectory.Name() {
									acceptProject = true
									break
								}
							}
							if !acceptProject {
								continue
							}
						}
						sectionNames, err := ioutil.ReadDir(dir + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name())
						if err != nil {
							logger.Printf("Couldn't read into: %s \n", projectDirectory.Name())
						}
						for _, sectionName := range sectionNames {
							sectionConfigFiles, err := ioutil.ReadDir(dir + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name())
							if err != nil {
								logger.Printf("Couldn't read into: %s \n", sectionName.Name())
							}
							for _, sectionConfigFile := range sectionConfigFiles {
								path := dir + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name() + "/" + sectionConfigFile.Name()
								SeedVaultFromFile(config, path, service, uploadCert)
							}
						}
					}
				}
				if strings.Count(fileSteppedInto.Name(), "_") >= 2 { //Check if file name is a versioned seed file -> 2 underscores "_"
					continue
				}

				if !normalEnv { //Enterprise ID
					dotSplit := strings.Split(strings.Split(fileSteppedInto.Name(), "_")[0], ".") //Checks if file name only has digits for enterprise
					if len(dotSplit) > 2 {
						_, err := strconv.Atoi(dotSplit[len(dotSplit)-1])
						if err != nil {
							continue
						}
					}
				}

				if !strings.HasSuffix(fileSteppedInto.Name(), "_seed.yml") { //Rigid file path check - must be env_seed.yml or dev.eid_seed.yml
					continue
				}

				if normalEnv && len(strings.Split(fileSteppedInto.Name(), ".")) > 2 {
					continue
				}

				if starEnv && len(strings.Split(fileSteppedInto.Name(), ".")) <= 2 {
					continue
				}

				ext := filepath.Ext(fileSteppedInto.Name())
				if strings.HasPrefix(fileSteppedInto.Name(), env) && (ext == ".yaml" || ext == ".yml") { // Only read YAML config files
					logger.Println("\t\t" + fileSteppedInto.Name())
					logger.Printf("\tFound seed file: %s\n", fileSteppedInto.Name())
					var path string
					if indexedEnvNot {
						path = dir + "/" + envDir.Name() + "/" + suffix + "/" + fileSteppedInto.Name()
					} else {
						path = dir + "/" + envDir.Name() + "/" + fileSteppedInto.Name()
					}
					logger.Println("\tSeeding vault with: " + fileSteppedInto.Name())

					SeedVaultFromFile(config, path, service, uploadCert)
					seeded = true
				}
			}
		}
	}
	if !seeded {
		eUtils.LogInfo(config, "Environment is not valid - Environment: "+env)
	}
	return nil
}

//SeedVaultFromFile takes a file path and seeds the vault with the seeds found in an individual file
func SeedVaultFromFile(config *utils.DriverConfig, filepath string, service string, uploadCert bool) {
	rawFile, err := ioutil.ReadFile(filepath)
	// Open file
	eUtils.LogErrorAndSafeExit(config, err, 1)
	SeedVaultFromData(config, strings.SplitAfterN(filepath, "/", 3)[2], rawFile, service, uploadCert)
}

//SeedVaultFromData takes file bytes and seeds the vault with contained data
func SeedVaultFromData(config *utils.DriverConfig, filepath string, fData []byte, service string, uploadCert bool) error {
	config.Log.SetPrefix("[SEED]")
	config.Log.Println("=========New File==========")
	var verificationData map[interface{}]interface{} // Create a reference for verification. Can't run until other secrets written
	// Unmarshal
	var rawYaml interface{}
	hasEmptyValues := bytes.Contains(fData, []byte("<Enter Secret Here>"))
	if hasEmptyValues && !strings.HasPrefix(filepath, "Index/") {
		return eUtils.LogAndSafeExit(config, "Incomplete configuration of seed data.  Found default secret data: '<Enter Secret Here>'.  Refusing to continue.", 1)
	}

	if strings.HasPrefix(filepath, "Restricted/") { //Fix incoming pathing for restricted projects
		i := strings.LastIndex(filepath, "/"+config.Env)
		filepath = filepath[:i]
	}

	err := yaml.Unmarshal(fData, &rawYaml)
	if err != nil {
		return eUtils.LogErrorAndSafeExit(config, err, 1)
	}

	seed, ok := rawYaml.(map[interface{}]interface{})
	if ok == false {
		return eUtils.LogAndSafeExit(config, "Invalid yaml file.  Refusing to continue.", 1)
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
				if !uploadCert {
					config.Log.Printf("Key with no value will not be written: %s\n", current.path+": "+k.(string))
				}
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
				if hasEmptyValues {
					// Scrub variables that were not initialized.
					removeKeys := []string{}
					switch v.(type) {
					case writeCollection:
						for dataKey, dataValue := range v.(writeCollection).data {
							if !strings.Contains(dataValue.(string), "<Enter Secret Here>") {
								removeKeys = append(removeKeys, dataKey)
							}
						}
						for _, removeKey := range removeKeys {
							delete(v.(writeCollection).data, removeKey)
						}
					}
				}
				writeVals.data[k.(string)] = v
				hasLeafNodes = true
			}
		}
		if hasLeafNodes { // Save all writable values in the current path
			writeStack = append(writeStack, writeVals)
		}
	}

	mod, err := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, nil, config.Log) // Connect to vault
	if err != nil {
		return eUtils.LogErrorAndSafeExit(config, err, 1)
	}
	mod.Env = config.Env
	if strings.HasPrefix(filepath, "Index/") || strings.HasPrefix(filepath, "Restricted/") { //Sets restricted to indexpath due to forward logic using indexpath
		mod.SectionPath = strings.TrimSuffix(filepath, "_seed.yml")
		config.Log.Println("Seeding configuration data for the following templates:" + mod.SectionPath)
	} else {
		config.Log.Println("Seeding configuration data for the following templates:" + filepath)
	}
	// Write values to vault
	config.Log.Println("Please verify that these templates exist in each service")

	for _, entry := range writeStack {
		// Output data being written
		// Write data and ouput any errors
		if strings.HasPrefix(entry.path, "values/") {
			if certPathData, certPathOk := entry.data["certSourcePath"]; certPathOk {
				if !uploadCert {
					continue
				}
				certPath := fmt.Sprintf("%s", certPathData)
				eUtils.LogInfo(config, fmt.Sprintf("Inspecting certificate: "+certPath+"."))

				if strings.Contains(certPath, "ENV") {
					if len(config.Env) >= 5 && (config.Env)[:5] == "local" {
						envParts := strings.SplitN(config.Env, "/", 3)
						certPath = strings.Replace(certPath, "ENV", envParts[1], 1)
					} else {
						certPath = strings.Replace(certPath, "ENV", config.Env, 1)
					}
				}
				certPath = "trc_seeds/" + certPath
				cert, err := ioutil.ReadFile(certPath)
				if err != nil {
					utils.LogErrorObject(config, err, false)
					continue
				}

				if err == nil {
					//if pfx file size greater than 25 KB, print warning
					if len(cert) > 32000 {
						eUtils.LogInfo(config, "Unreasonable size for certificate type file. Not written to vault")
						continue
					}

					isValidCert := false
					var certValidationErr error
					if strings.HasSuffix(certPath, ".pfx") {
						eUtils.LogInfo(config, "Inspecting pfx: "+certPath+".")
						isValidCert, certValidationErr = validator.IsPfxRfc7292(cert)
					} else if strings.HasSuffix(certPath, ".cer") {
						eUtils.LogInfo(config, "Inspecting cer: "+certPath+".")
						cert, certValidationErr := x509.ParseCertificate(cert)
						if certValidationErr == nil {
							isValidCert = true
						} else {
							eUtils.LogInfo(config, "failed to parse and verify certificate: "+certValidationErr.Error())
						}
						var certHost string
						if certHostData, certHostOk := entry.data["certHost"]; certHostOk {
							certHost = fmt.Sprintf("%s", certHostData)
						} else {
							eUtils.LogInfo(config, "Missing certHost, cannot validate cert.  Not written to vault")
							continue
						}
						switch config.Env {
						case "dev":
							certHost = strings.Replace(certHost, "*", "develop", 1)
							break
						case "QA":
							certHost = strings.Replace(certHost, "*", "qa", 1)
							break
						case "performance":
							certHost = strings.Replace(certHost, "*", "performance", 1)
							break
						}

						opts := x509.VerifyOptions{
							DNSName: certHost,
						}

						if _, err := cert.Verify(opts); err != nil {
							if _, isUnknownAuthority := err.(x509.UnknownAuthorityError); !isUnknownAuthority {
								eUtils.LogInfo(config, "Unknown authority: failed to verify certificate: "+err.Error())
								continue
							}
						}
					} else if strings.HasSuffix(certPath, ".pem") {
						eUtils.LogInfo(config, "Inspecting pem: "+certPath+".")
						pemBlock, _ := pem.Decode(cert)
						if pemBlock == nil {
							eUtils.LogInfo(config, "failed to verify certificate PEM.")
						} else {
							isValidCert = true
						}
					} else if strings.HasSuffix(certPath, ".jks") {
						isValidCert = true
					}
					if isValidCert {
						eUtils.LogInfo(config, "Certificate passed validation: "+certPath+".")
						certBase64 := base64.StdEncoding.EncodeToString(cert)
						if _, ok := entry.data["certData"]; ok {
							// insecure value entry.
							entry.data["certData"] = certBase64
							eUtils.LogInfo(config, "Public cert updated: "+certPath+".")
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
											WriteData(config, secretEntry.path, secretEntry.data, mod)
											WriteData(config, entry.path, entry.data, mod)
											done = true
											break
										}
									}
								}
								eUtils.LogInfo(config, "Cert loaded from: "+certPath+".")

								if done {
									continue
								}
							}
						}
					} else {
						eUtils.LogInfo(config, "Cert validation failure.  Cert will not be loaded."+certValidationErr.Error())
						delete(entry.data, "certData")
						delete(entry.data, "certHost")
						delete(entry.data, "certSourcePath")
						delete(entry.data, "certDestPath")
						continue
					}
				} else {
					eUtils.LogInfo(config, "Missing expected cert at: "+certPath+".  Cert will not be loaded.")
					continue
				}
			} else {
				if uploadCert {
					// Skip non-certs.
					continue
				}
			}
		} else {
			_, certPathOk := entry.data["certSourcePath"]
			_, certDataOK := entry.data["certData"]

			if certPathOk || certDataOK {
				if !uploadCert {
					continue
				}
			} else {
				if uploadCert {
					// Skip non-certs.
					continue
				}
			}
		}

		if service != "" {
			if strings.HasSuffix(entry.path, service) || strings.Contains(entry.path, "Common") {
				WriteData(config, entry.path, entry.data, mod)
			}
		} else {
			//			/Index/TrcVault/regionId/<regionEnv>
			WriteData(config, entry.path, entry.data, mod)
		}
	}

	// Run verification after seeds have been written
	warn, err := verify(config, mod, verificationData)
	utils.LogErrorObject(config, err, false)
	utils.LogWarningsObject(config, warn, false)
	eUtils.LogInfo(config, "\nInitialization complete for "+mod.Env+".\n")
	return nil
}

//WriteData takes entry path and date from each iteration of writeStack in SeedVaultFromData and writes to vault
func WriteData(config *eUtils.DriverConfig, path string, data map[string]interface{}, mod *kv.Modifier) {
	root := strings.Split(path, "/")[0]
	if templateWritten == nil {
		templateWritten = make(map[string]bool)
	}
	if root == "templates" { //Check if templates have already been written in this init call.
		_, ok := templateWritten[path]
		if !ok {
			templateWritten[path] = true
		} else {
			return
		}
	}

	warn, err := mod.Write(path, data)

	utils.LogWarningsObject(config, warn, false)
	utils.LogErrorObject(config, err, false)
	// Update value metrics to reflect credential use
	if root == "templates" {
		//Printing out path of each entry so that users can verify that folder structure in seed files are correct
		config.Log.Println("trc_" + path + ".*.tmpl")
		for _, v := range data {
			if templateKey, ok := v.([]interface{}); ok {
				metricsKey := templateKey[0].(string) + "." + templateKey[1].(string)
				mod.AdjustValue("value-metrics/credentials", metricsKey, 1)
			}
		}
	}
}
