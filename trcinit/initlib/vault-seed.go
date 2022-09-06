package initlib

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"tierceron/buildopts/coreopts"
	"tierceron/trcx/xutil"
	"tierceron/validator"
	helperkv "tierceron/vaulthelper/kv"

	eUtils "tierceron/utils"

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
func SeedVault(config *eUtils.DriverConfig) error {

	config.Log.SetPrefix("[SEED]")
	config.Log.Printf("Seeding vault from seeds in: %s\n", config.StartDir[0])

	files, err := ioutil.ReadDir(config.StartDir[0])

	templateWritten = make(map[string]bool)
	if len(files) == 1 && files[0].Name() == "certs" && config.WantCerts {
		// Cert rotation support without templates
		config.Log.Printf("No templates available, Common service requested.: %s\n", config.StartDir[0])

		var templatePaths = coreopts.GetSupportedTemplates()
		regions := []string{}

		if strings.HasPrefix(config.Env, "staging") || strings.HasPrefix(config.Env, "prod") || strings.HasPrefix(config.Env, "dev") {
			regions = eUtils.GetSupportedProdRegions()
		}
		config.Regions = regions

		_, _, seedData, errGenerateSeeds := xutil.GenerateSeedsFromVaultRaw(config, true, templatePaths)
		if errGenerateSeeds != nil {
			return eUtils.LogErrorAndSafeExit(config, errGenerateSeeds, -1)
		}

		seedData = strings.ReplaceAll(seedData, "<Enter Secret Here>", "")

		SeedVaultFromData(config, "", []byte(seedData))
		return nil
	}

	eUtils.LogErrorObject(config, err, true)

	_, suffix, indexedEnvNot, _ := helperkv.PreCheckEnvironment(config.Env)

	seeded := false
	starEnv := false
	if strings.Contains(config.Env, "*") {
		starEnv = true
		config.Env = strings.Split(config.Env, "*")[0]
	}
	for _, envDir := range files {
		if strings.HasPrefix(config.Env, envDir.Name()) || (strings.HasPrefix(config.Env, "local") && envDir.Name() == "local") {
			config.Log.Println("\tStepping into: " + envDir.Name())
			var filesSteppedInto []fs.FileInfo
			if indexedEnvNot {
				filesSteppedInto, err = ioutil.ReadDir(config.StartDir[0] + "/" + envDir.Name() + "/" + suffix)
			} else {
				filesSteppedInto, err = ioutil.ReadDir(config.StartDir[0] + "/" + envDir.Name())
			}
			eUtils.LogErrorObject(config, err, true)

			conflictingFile := false
			for _, fileSteppedInto := range filesSteppedInto {
				if !strings.HasPrefix(fileSteppedInto.Name(), config.Env) {
					if strings.Contains(config.Env, ".") {
						secondCheck := strings.Split(config.Env, ".")[0]
						if !strings.HasPrefix(fileSteppedInto.Name(), secondCheck) {
							conflictingFile = true
							config.Log.Printf("Found conflicting env seed file: %s \n", fileSteppedInto.Name())
						}
					}
				}
			}
			if len(filesSteppedInto) > 1 && conflictingFile {
				eUtils.CheckWarning(config, fmt.Sprintf("Multiple potentially conflicting configuration files found for environment: %s", envDir.Name()), true)
			}

			normalEnv := false
			if !starEnv && !strings.Contains(config.Env, ".") {
				normalEnv = true
			}

			for _, fileSteppedInto := range filesSteppedInto {
				if fileSteppedInto.Name() == "Index" || fileSteppedInto.Name() == "Restricted" {
					projectDirectories, err := ioutil.ReadDir(config.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name())
					if err != nil {
						config.Log.Printf("Couldn't read into: %s \n", fileSteppedInto.Name())
					}
					// Iterate of projects...
					for _, projectDirectory := range projectDirectories {
						if len(config.ProjectSections) > 0 {
							acceptProject := false
							for _, index := range config.ProjectSections {
								if index == projectDirectory.Name() {
									acceptProject = true
									break
								}
							}
							if !acceptProject {
								continue
							}
						}
						sectionNames, err := ioutil.ReadDir(config.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name())
						if err != nil {
							config.Log.Printf("Couldn't read into: %s \n", projectDirectory.Name())
						}
						for _, sectionName := range sectionNames {
							sectionConfigFiles, err := ioutil.ReadDir(config.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name())
							if err != nil {
								config.Log.Printf("Couldn't read into: %s \n", sectionName.Name())
							}
							for _, sectionConfigFile := range sectionConfigFiles {
								path := config.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name() + "/" + sectionConfigFile.Name()
								if strings.HasPrefix(sectionConfigFile.Name(), ".") || (config.SubSectionValue != "" && (sectionConfigFile.Name() != config.SubSectionValue)) {
									continue
								}
								subSectionConfigFiles, err := ioutil.ReadDir(path)
								if err != nil {
									config.Log.Printf("Couldn't read into: %s \n", config.SubSectionName)
								}

								if len(subSectionConfigFiles) > 0 {
									for _, subSectionConfigFile := range subSectionConfigFiles {
										subSectionPath := config.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name() + "/" + sectionConfigFile.Name() + "/" + subSectionConfigFile.Name()
										if strings.HasPrefix(sectionConfigFile.Name(), ".") ||
											(config.SubSectionName != "" && (!strings.HasPrefix("/"+subSectionConfigFile.Name(), config.SubSectionName))) ||
											(config.SectionName != "" && (!strings.HasPrefix("/"+sectionName.Name()+"/", "/"+config.SectionName+"/"))) {
											continue
										}
										SeedVaultFromFile(config, subSectionPath)
										seeded = true
									}
								} else {
									SeedVaultFromFile(config, path)
									seeded = true
								}
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

				if seeded {
					continue
				}

				ext := filepath.Ext(fileSteppedInto.Name())
				if strings.HasPrefix(fileSteppedInto.Name(), config.Env) && (ext == ".yaml" || ext == ".yml") { // Only read YAML config files
					config.Log.Println("\t\t" + fileSteppedInto.Name())
					config.Log.Printf("\tFound seed file: %s\n", fileSteppedInto.Name())
					var path string
					if indexedEnvNot {
						path = config.StartDir[0] + "/" + envDir.Name() + "/" + suffix + "/" + fileSteppedInto.Name()
					} else {
						path = config.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name()
					}
					config.Log.Println("\tSeeding vault with: " + fileSteppedInto.Name())

					SeedVaultFromFile(config, path)
					seeded = true
				}
			}
		}
	}
	if !seeded {
		eUtils.LogInfo(config, "Environment is not valid - Environment: "+config.Env)
	} else {
		eUtils.LogInfo(config, "Initialization complete for: "+config.Env)
	}
	return nil
}

//SeedVaultFromFile takes a file path and seeds the vault with the seeds found in an individual file
func SeedVaultFromFile(config *eUtils.DriverConfig, filepath string) {
	rawFile, err := ioutil.ReadFile(filepath)
	// Open file
	eUtils.LogErrorAndSafeExit(config, err, 1)
	eUtils.LogInfo(config, "Seed written to vault from "+filepath)
	SeedVaultFromData(config, strings.SplitAfterN(filepath, "/", 3)[2], rawFile)
}

//SeedVaultFromData takes file bytes and seeds the vault with contained data
func SeedVaultFromData(config *eUtils.DriverConfig, filepath string, fData []byte) error {
	config.Log.SetPrefix("[SEED]")
	config.Log.Println("=========New File==========")
	var verificationData map[interface{}]interface{} // Create a reference for verification. Can't run until other secrets written
	// Unmarshal
	var rawYaml interface{}
	hasEmptyValues := bytes.Contains(fData, []byte("<Enter Secret Here>"))
	isIndexData := strings.HasPrefix(filepath, "Index/") || strings.Contains(filepath, "/PublicIndex/")

	if hasEmptyValues && !isIndexData {
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
				if !config.WantCerts {
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

	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, nil, config.Log) // Connect to vault
	if err != nil {
		return eUtils.LogErrorAndSafeExit(config, err, 1)
	}
	mod.Env = config.Env
	if strings.Contains(filepath, "/PublicIndex/") {
		config.Log.Println("Seeding configuration data for the following templates: DataStatistics")
	} else if isIndexData || strings.HasPrefix(filepath, "Restricted/") { //Sets restricted to indexpath due to forward logic using indexpath
		mod.SectionPath = strings.TrimSuffix(filepath, "_seed.yml")
		if len(config.IndexFilter) > 0 && isIndexData {
			mod.SectionPath = mod.SectionPath[:strings.LastIndex(mod.SectionPath, "/")+1] + config.IndexFilter[0] + mod.SectionPath[strings.LastIndex(mod.SectionPath, "/"):] //This is for mysqlFile
		}
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
				if !config.WantCerts {
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
				certPath = coreopts.GetFolderPrefix() + "_seeds/" + certPath
				cert, err := ioutil.ReadFile(certPath)
				if err != nil {
					eUtils.LogErrorObject(config, err, false)
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
						if certValidationErr == nil {
							eUtils.LogInfo(config, "Cert validation failure. Cert will not be loaded.")
						} else {
							eUtils.LogInfo(config, "Cert validation failure.  Cert will not be loaded."+certValidationErr.Error())
						}
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
				if config.WantCerts {
					// Skip non-certs.
					continue
				}
			}
		} else {
			_, certPathOk := entry.data["certSourcePath"]
			_, certDataOK := entry.data["certData"]

			if certPathOk || certDataOK {
				if !config.WantCerts {
					continue
				}
			} else {
				if config.WantCerts {
					// Skip non-certs.
					continue
				}
			}
		}

		// Write Secrets...

		// TODO: Support all services, so range over ServicesWanted....
		// Populate as a slice...
		if config.ServicesWanted[0] != "" {
			if strings.HasSuffix(entry.path, config.ServicesWanted[0]) || strings.Contains(entry.path, "Common") {
				WriteData(config, entry.path, entry.data, mod)
			}
		} else if strings.Contains(filepath, "/PublicIndex/") && !strings.Contains(entry.path, "templates") {
			WriteData(config, filepath, entry.data, mod)
		} else {
			//			/Index/TrcVault/regionId/<regionEnv>
			WriteData(config, entry.path, entry.data, mod)
		}
	}

	// Run verification after seeds have been written
	warn, err := verify(config, mod, verificationData)
	eUtils.LogErrorObject(config, err, false)
	eUtils.LogWarningsObject(config, warn, false)
	if !isIndexData {
		eUtils.LogInfo(config, "\nInitialization complete for "+mod.Env+".\n")
	}

	return nil
}

//WriteData takes entry path and date from each iteration of writeStack in SeedVaultFromData and writes to vault
func WriteData(config *eUtils.DriverConfig, path string, data map[string]interface{}, mod *helperkv.Modifier) {
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

	eUtils.LogWarningsObject(config, warn, false)
	eUtils.LogErrorObject(config, err, false)
	// Update value metrics to reflect credential use
	if root == "templates" {
		//Printing out path of each entry so that users can verify that folder structure in seed files are correct
		config.Log.Println(coreopts.GetFolderPrefix() + "_" + path + ".*.tmpl")
		for _, v := range data {
			if templateKey, ok := v.([]interface{}); ok {
				metricsKey := templateKey[0].(string) + "." + templateKey[1].(string)
				mod.AdjustValue("value-metrics/credentials", metricsKey, 1)
			}
		}
	}
}
