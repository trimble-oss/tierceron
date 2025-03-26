package initlib

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template/parse"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/validator"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

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

func GetTemplateParam(driverConfig *config.DriverConfig, mod *helperkv.Modifier, filePath string, paramWanted string) (string, error) {

	templateEncoded, err := vcutils.GetTemplate(driverConfig, mod, filePath)
	if err != nil {
		return "", err
	}
	templateBytes, dcErr := base64.StdEncoding.DecodeString(templateEncoded)
	if dcErr != nil {
		return "", dcErr
	}

	templateStr := string(templateBytes)

	t := template.New("template")
	t, err = t.Parse(templateStr)
	if err != nil {
		return "", err
	}
	commandList := t.Tree.Root

	for _, node := range commandList.Nodes {
		if node.Type() == parse.NodeAction {
			var args []string
			fields := node.(*parse.ActionNode).Pipe
			for _, arg := range fields.Cmds[0].Args {
				templateParameter := strings.ReplaceAll(arg.String(), "\"", "")

				if len(args) > 0 && args[len(args)-1] == paramWanted {
					return templateParameter, nil
				}

				args = append(args, templateParameter)
			}
		}
	}

	return "", errors.New("Could not find the param " + paramWanted + " in this template " + filePath + ".")
}

// SeedVault seeds the vault with seed files in the given directory -> only init uses this
func SeedVault(driverConfig *config.DriverConfig) error {

	driverConfig.CoreConfig.Log.SetPrefix("[SEED]")
	driverConfig.CoreConfig.Log.Printf("Seeding vault from seeds in: %s\n", driverConfig.StartDir[0])

	files, err := os.ReadDir(driverConfig.StartDir[0])
	if len(files) == 0 {
		fmt.Println("Empty seed file directory")
		return eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, errors.New("Missing seed data."), -1)
	}

	if len(driverConfig.FileFilter) == 1 && driverConfig.FileFilter[0] == "nest" {
		dynamicPathFilter := ""
		if driverConfig.CoreConfig.DynamicPathFilter != "" {
			dynamicPathFilter = driverConfig.StartDir[0] + "/" + driverConfig.CoreConfig.Env + "/" + driverConfig.CoreConfig.DynamicPathFilter
		}
		err := filepath.Walk(driverConfig.StartDir[0]+"/"+driverConfig.CoreConfig.Env,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if dynamicPathFilter != "" {
					if strings.HasPrefix(path, dynamicPathFilter) && strings.HasSuffix(path, "_seed.yml") {
						SeedVaultFromFile(driverConfig, path)
					}
				} else {
					if strings.HasSuffix(path, "_seed.yml") {
						SeedVaultFromFile(driverConfig, path)
					}
				}
				return nil
			})
		if err != nil {
			driverConfig.CoreConfig.Log.Println(err)
		}
		fmt.Println("Nested initialization complete")
		return nil
	}
	templateWritten = make(map[string]bool)
	//
	// The following logic section for server based certificate loading is for when it is known that
	// all templates for the certs exist in vault.
	//
	// For general certificate loading (where templates may not have yet been pushed to vault)
	// a separate path deeper into the code is used for certificate loading.
	//
	if driverConfig.IsShellSubProcess && driverConfig.CoreConfig.WantCerts && (len(files) > 1 || (files[0].Name() != "certs")) {
		fmt.Println("Unusual deployment cert configuration.  Refusing to continue...")
		return eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, errors.New("Invalid deployment cert configuration.  Refusing to continue..."), -1)
	}

	if len(files) == 1 && files[0].Name() == "certs" && driverConfig.CoreConfig.WantCerts {
		// Cert rotation support without templates
		driverConfig.CoreConfig.Log.Printf("Initializing certificates.  Common service requested.: %s\n", driverConfig.StartDir[0])

		var templatePaths = coreopts.BuildOptions.GetSupportedTemplates(driverConfig.StartDir)
		regions := []string{}

		if strings.HasPrefix(driverConfig.CoreConfig.Env, "staging") || strings.HasPrefix(driverConfig.CoreConfig.Env, "prod") || strings.HasPrefix(driverConfig.CoreConfig.Env, "dev") {
			regions = eUtils.GetSupportedProdRegions()
		}
		driverConfig.CoreConfig.Regions = regions

		var tempPaths []string
		for _, templatePath := range templatePaths {
			var err error
			tokenName := fmt.Sprintf("config_token_%s_unrestricted", driverConfig.CoreConfig.EnvBasis)
			if driverConfig.CoreConfig.CurrentTokenNamePtr != nil &&
				driverConfig.CoreConfig.TokenCache.GetToken(*driverConfig.CoreConfig.CurrentTokenNamePtr) != nil {
				tokenName = *driverConfig.CoreConfig.CurrentTokenNamePtr
			}

			mod, err := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, driverConfig.CoreConfig.EnvBasis, true)
			if err != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
				continue
			}

			mod.Env = driverConfig.CoreConfig.Env
			if len(driverConfig.ProjectSections) > 0 {
				mod.ProjectIndex = driverConfig.ProjectSections
				mod.EnvBasis = strings.Split(driverConfig.CoreConfig.EnvBasis, "_")[0]
				mod.SectionName = driverConfig.SectionName
				mod.SubSectionValue = driverConfig.SubSectionValue
			}
			templateParam, tParamErr := GetTemplateParam(driverConfig, mod, templatePath, ".certSourcePath")
			if tParamErr != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, tParamErr, false)
				continue
			}

			if driverConfig.CoreConfig.EnvBasis == "" {
				driverConfig.CoreConfig.EnvBasis = strings.Split(driverConfig.CoreConfig.Env, "_")[0]
			}
			templateParam = strings.Replace(templateParam, "ENV", driverConfig.CoreConfig.EnvBasis, -1)
			wd, err := os.Getwd()
			if err != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, errors.New("could not get working directory for cert existence verification"), false)
				continue
			}

			_, fileError := os.Stat(wd + "/" + coreopts.BuildOptions.GetFolderPrefix(nil) + "_seeds/" + templateParam)
			if fileError != nil {
				if os.IsNotExist(fileError) {
					eUtils.LogErrorObject(driverConfig.CoreConfig, errors.New("File does not exist\n"+templateParam), false)
					continue
				}
			} else {
				tempPaths = append(tempPaths, templatePath)
			}
		}
		if len(tempPaths) > 0 {
			templatePaths = tempPaths
		} else {
			return eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, errors.New("no valid cert files were located"), -1)
		}
		_, _, seedData, errGenerateSeeds := xutil.GenerateSeedsFromVaultRaw(driverConfig, true, templatePaths)
		if errGenerateSeeds != nil {
			return eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, errGenerateSeeds, -1)
		}

		driverConfig.ServiceFilter = templatePaths
		seedData = strings.ReplaceAll(seedData, "<Enter Secret Here>", "")

		seedErr := SeedVaultFromData(driverConfig, "", []byte(seedData))
		eUtils.LogErrorObject(driverConfig.CoreConfig, seedErr, true)
		if seedErr != nil {
			return seedErr
		}
		return nil
	}

	eUtils.LogErrorObject(driverConfig.CoreConfig, err, true)

	_, suffix, indexedEnvNot, _ := helperkv.PreCheckEnvironment(driverConfig.CoreConfig.Env)

	seeded := false
	starEnv := false
	if strings.Contains(driverConfig.CoreConfig.Env, "*") {
		starEnv = true
		driverConfig.CoreConfig.Env = strings.Split(driverConfig.CoreConfig.Env, "*")[0]
	}
	for _, envDir := range files {
		if strings.HasPrefix(driverConfig.CoreConfig.Env, envDir.Name()) || (strings.HasPrefix(driverConfig.CoreConfig.Env, "local") && envDir.Name() == "local") || (driverConfig.CoreConfig.WantCerts && strings.HasPrefix(envDir.Name(), "certs")) {
			if driverConfig.CoreConfig.Env != driverConfig.CoreConfig.EnvBasis && driverConfig.CoreConfig.Env != envDir.Name() { //If raw & env don't match -> current env is env-* so env will be skipped
				continue
			}

			driverConfig.CoreConfig.Log.Println("\tStepping into: " + envDir.Name())

			if driverConfig.CoreConfig.DynamicPathFilter != "" {
				sectionConfigFiles, err := os.ReadDir(driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + driverConfig.CoreConfig.DynamicPathFilter)
				if err != nil {
					driverConfig.CoreConfig.Log.Printf("Seed Sections Couldn't read into: %s \n", driverConfig.CoreConfig.DynamicPathFilter)
				}
				seedFileCount := 0
				var seedFileName string
				for _, sectionConfigFile := range sectionConfigFiles {
					if strings.HasSuffix(sectionConfigFile.Name(), ".yml") {
						seedFileName = sectionConfigFile.Name()
						seedFileCount++
					}
				}

				if seedFileCount > 1 {
					eUtils.CheckWarning(driverConfig.CoreConfig, fmt.Sprintf("Multiple potentially conflicting configuration files found for environment: %s", envDir.Name()), true)
				}

				SeedVaultFromFile(driverConfig, driverConfig.StartDir[0]+"/"+envDir.Name()+"/"+driverConfig.CoreConfig.DynamicPathFilter+"/"+seedFileName)
				seeded = true
				continue
			}

			var filesSteppedInto []fs.DirEntry
			if indexedEnvNot {
				filesSteppedInto, err = os.ReadDir(driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + suffix)
			} else {
				filesSteppedInto, err = os.ReadDir(driverConfig.StartDir[0] + "/" + envDir.Name())
			}
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, true)

			conflictingFile := false
			for _, fileSteppedInto := range filesSteppedInto {
				if !strings.HasPrefix(fileSteppedInto.Name(), driverConfig.CoreConfig.Env) {
					if strings.Contains(driverConfig.CoreConfig.Env, ".") {
						secondCheck := strings.Split(driverConfig.CoreConfig.Env, ".")[0]
						if !strings.HasPrefix(fileSteppedInto.Name(), secondCheck) {
							conflictingFile = true
							driverConfig.CoreConfig.Log.Printf("Found conflicting env seed file: %s \n", fileSteppedInto.Name())
						}
					}
				}
			}
			if len(filesSteppedInto) > 1 && conflictingFile {
				eUtils.CheckWarning(driverConfig.CoreConfig, fmt.Sprintf("Multiple potentially conflicting configuration files found for environment: %s", envDir.Name()), true)
			}

			normalEnv := false
			if !starEnv && !strings.Contains(driverConfig.CoreConfig.Env, ".") {
				normalEnv = true
			}

			for _, fileSteppedInto := range filesSteppedInto {
				if strings.HasSuffix(fileSteppedInto.Name(), ".yml") {
					if !*eUtils.BasePtr {
						continue
					}
					SeedVaultFromFile(driverConfig, driverConfig.StartDir[0]+"/"+envDir.Name()+"/"+fileSteppedInto.Name())
					seeded = true
				} else if fileSteppedInto.Name() == "Index" || fileSteppedInto.Name() == "Restricted" || fileSteppedInto.Name() == "Protected" {
					if eUtils.OnlyBasePtr {
						continue
					}
					projectDirectories, err := os.ReadDir(driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name())
					if err != nil {
						driverConfig.CoreConfig.Log.Printf("Projects Couldn't read into: %s \n", fileSteppedInto.Name())
					}
					// Iterate of projects...
					for _, projectDirectory := range projectDirectories {
						if len(driverConfig.ProjectSections) > 0 {
							acceptProject := false
							for _, index := range driverConfig.ProjectSections {
								if index == projectDirectory.Name() {
									acceptProject = true
									break
								}
							}
							if !acceptProject {
								continue
							}
						}
						sectionNames, err := os.ReadDir(driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name())
						if err != nil {
							driverConfig.CoreConfig.Log.Printf("Sections Couldn't read into: %s \n", projectDirectory.Name())
						}
						for _, sectionName := range sectionNames {
							if driverConfig.SectionName != "" && sectionName.Name() != driverConfig.SectionName {
								continue
							}

							sectionConfigFiles, err := os.ReadDir(driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name())
							if err != nil {
								driverConfig.CoreConfig.Log.Printf("Section Config Couldn't read into: %s \n", sectionName.Name())
							}

							for _, sectionConfigFile := range sectionConfigFiles {
								path := driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name() + "/" + sectionConfigFile.Name()
								if strings.HasPrefix(sectionConfigFile.Name(), ".") || (driverConfig.SubSectionValue != "" && (sectionConfigFile.Name() != driverConfig.SubSectionValue)) {
									continue
								}
								subSectionConfigFiles, err := os.ReadDir(path)

								if err != nil {
									driverConfig.CoreConfig.Log.Printf("Sub Sections Couldn't read into: %s \n", driverConfig.SubSectionName)
								}

								if len(subSectionConfigFiles) > 0 {
									for _, subSectionConfigFile := range subSectionConfigFiles {
										subSectionPath := driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name() + "/" + projectDirectory.Name() + "/" + sectionName.Name() + "/" + sectionConfigFile.Name() + "/" + subSectionConfigFile.Name()
										if strings.HasPrefix(sectionConfigFile.Name(), ".") ||
											(driverConfig.SubSectionName != "" && (!strings.HasPrefix("/"+subSectionConfigFile.Name(), driverConfig.SubSectionName))) ||
											(driverConfig.SectionName != "" && (!strings.HasPrefix("/"+sectionName.Name()+"/", "/"+driverConfig.SectionName+"/"))) {
											continue
										}

										if subSectionConfigFile.IsDir() {
											deepNestedFiles, err := os.ReadDir(subSectionPath)
											if err != nil {
												driverConfig.CoreConfig.Log.Printf("Deep Nested Couldn't read into: %s \n", driverConfig.SubSectionName)
												continue
											}

											for _, deepNestedFile := range deepNestedFiles {
												if deepNestedFile.IsDir() {
													subSectionPath = subSectionPath + "/" + deepNestedFile.Name()
													deeplyNestedFiles, err := os.ReadDir(subSectionPath)
													if err != nil {
														driverConfig.CoreConfig.Log.Printf("Sub secting deep nested Couldn't read into: %s \n", driverConfig.SubSectionName)
														continue
													}
													for _, deeplyNestedFile := range deeplyNestedFiles {
														if !deeplyNestedFile.IsDir() {
															subSectionPath = subSectionPath + "/" + deeplyNestedFile.Name()
															SeedVaultFromFile(driverConfig, subSectionPath)
															seeded = true
														}
													}
												} else {
													subSectionPath = subSectionPath + "/" + deepNestedFile.Name()
													SeedVaultFromFile(driverConfig, subSectionPath)
													seeded = true
												}
											}
										} else {
											SeedVaultFromFile(driverConfig, subSectionPath)
											seeded = true
										}
									}
								} else {
									if len(driverConfig.ServiceFilter) > 0 {
										for _, filter := range driverConfig.ServiceFilter {
											if strings.HasSuffix(path, filter+"_seed.yml") {
												SeedVaultFromFile(driverConfig, driverConfig.StartDir[0]+"/"+envDir.Name()+"/"+fileSteppedInto.Name()+"/"+projectDirectory.Name()+"/"+sectionName.Name()+"/"+sectionConfigFile.Name())
												seeded = true
											}
										}
									} else {
										SeedVaultFromFile(driverConfig, path)
										seeded = true
									}
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
				if strings.HasPrefix(fileSteppedInto.Name(), driverConfig.CoreConfig.Env) && (ext == ".yaml" || ext == ".yml") { // Only read YAML config files
					driverConfig.CoreConfig.Log.Println("\t\t" + fileSteppedInto.Name())
					driverConfig.CoreConfig.Log.Printf("\tFound seed file: %s\n", fileSteppedInto.Name())
					var path string
					if indexedEnvNot {
						path = driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + suffix + "/" + fileSteppedInto.Name()
					} else {
						path = driverConfig.StartDir[0] + "/" + envDir.Name() + "/" + fileSteppedInto.Name()
					}
					driverConfig.CoreConfig.Log.Println("\tSeeding vault with: " + fileSteppedInto.Name())

					SeedVaultFromFile(driverConfig, path)
					seeded = true
				}
			}
		}
	}
	if !seeded {
		eUtils.LogInfo(driverConfig.CoreConfig, "Environment is not valid - Environment: "+driverConfig.CoreConfig.Env)
	} else {
		eUtils.LogInfo(driverConfig.CoreConfig, "\nInitialization complete for: "+driverConfig.CoreConfig.Env+"\n")
	}
	return nil
}

// SeedVaultFromFile takes a file path and seeds the vault with the seeds found in an individual file
func SeedVaultFromFile(driverConfig *config.DriverConfig, filepath string) {
	rawFile, err := os.ReadFile(filepath)
	// Open file
	eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, err, 1)
	if driverConfig.CoreConfig.WantCerts && (strings.Contains(filepath, "/Index/") || strings.Contains(filepath, "/PublicIndex/") || strings.Contains(filepath, "/Restricted/")) {
		driverConfig.CoreConfig.Log.Println("Skipping index: " + filepath + " Certs not allowed within index data.")
		return
	}

	eUtils.LogInfo(driverConfig.CoreConfig, "Seed written to vault from "+filepath)
	if len(driverConfig.ServiceFilter) > 0 && !strings.Contains(filepath, driverConfig.ServiceFilter[0]) {
		lastSlashIndex := strings.LastIndex(filepath, "/")
		filepath = filepath[:lastSlashIndex] + "/" + driverConfig.ServiceFilter[0] + "/" + filepath[lastSlashIndex+1:]
	}
	SeedVaultFromData(driverConfig, strings.SplitAfterN(filepath, "/", 3)[2], rawFile)
}

// seedVaultWithCertsFromEntry takes entry from writestack and if it contains a cert, writes it to vault.
func seedVaultWithCertsFromEntry(driverConfig *config.DriverConfig, mod *helperkv.Modifier, writeStack *[]writeCollection, entry *writeCollection) {
	certPathData, certPathOk := entry.data["certSourcePath"]
	if !certPathOk {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, "Missing cert path.", false)
		return
	}

	certPath := fmt.Sprintf("%s", certPathData)

	if strings.Contains(certPath, "ENV") {
		if len(driverConfig.CoreConfig.EnvBasis) >= 5 && (driverConfig.CoreConfig.EnvBasis)[:5] == "local" {
			envParts := strings.SplitN(driverConfig.CoreConfig.Env, "/", 3)
			certPath = strings.Replace(certPath, "ENV", envParts[1], 1)
		} else {
			certPath = strings.Replace(certPath, "ENV", driverConfig.CoreConfig.EnvBasis, 1)
		}
	}
	if strings.Contains(certPath, "..") {
		errMsg := eUtils.SanitizeForLogging("Invalid cert path: " + certPath + " Certs not allowed to contain complex path navigation.")
		fmt.Println(errMsg)
		driverConfig.CoreConfig.Log.Println(errMsg)
		return
	}
	certPath = coreopts.BuildOptions.GetFolderPrefix(nil) + "_seeds/" + certPath
	eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Inspecting certificate: "+certPath+"."))
	cert, err := os.ReadFile(certPath)
	if err != nil {
		eUtils.LogInfo(driverConfig.CoreConfig, "Missing expected cert at: "+certPath+".  Cert will not be loaded.")
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return
	} else {
		//if pfx file size greater than 25 KB, print warning
		if len(cert) > 32000 {
			eUtils.LogInfo(driverConfig.CoreConfig, "Unreasonable size for certificate type file. Not written to vault")
			return
		}

		isValidCert := false
		var certValidationErr error
		if strings.HasSuffix(certPath, ".pfx") {
			eUtils.LogInfo(driverConfig.CoreConfig, "Inspecting pfx: "+certPath+".")
			isValidCert, certValidationErr = validator.IsPfxRfc7292(cert)
		} else if strings.HasSuffix(certPath, ".cer") {
			eUtils.LogInfo(driverConfig.CoreConfig, "Inspecting cer: "+certPath+".")
			cert, certValidationErr := x509.ParseCertificate(cert)
			if certValidationErr == nil {
				isValidCert = true
			} else {
				eUtils.LogInfo(driverConfig.CoreConfig, "failed to parse and verify certificate: "+certValidationErr.Error())
			}
			var certHost string
			if certHostData, certHostOk := entry.data["certHost"]; certHostOk {
				certHost = fmt.Sprintf("%s", certHostData)
			} else {
				eUtils.LogInfo(driverConfig.CoreConfig, "Missing certHost, cannot validate cert.  Not written to vault")
				return
			}
			switch driverConfig.CoreConfig.EnvBasis {
			case "itdev":
				fallthrough
			case "dev":
				certHost = strings.Replace(certHost, "*", "develop", 1)
			case "QA":
				certHost = strings.Replace(certHost, "*", "qa", 1)
			case "RQA":
				certHost = strings.Replace(certHost, "*", "qa", 1)
			case "auto":
				certHost = strings.Replace(certHost, "*", "qa", 1)
			case "performance":
				certHost = strings.Replace(certHost, "*", "performance", 1)
			}

			opts := x509.VerifyOptions{
				DNSName: certHost,
			}

			if _, err := cert.Verify(opts); err != nil {
				if _, isUnknownAuthority := err.(x509.UnknownAuthorityError); !isUnknownAuthority {
					eUtils.LogInfo(driverConfig.CoreConfig, "Seeding of requested cert failed because it is invalid: Unknown authority: failed to verify certificate: "+err.Error())
					return
				}
			}
		} else if strings.HasSuffix(certPath, ".crt") || strings.HasSuffix(certPath, ".key") {
			eUtils.LogInfo(driverConfig.CoreConfig, "Inspecting crt or key: "+certPath+".")
			pemBlock, _ := pem.Decode(cert)
			if pemBlock == nil {
				eUtils.LogInfo(driverConfig.CoreConfig, "failed to verify certificate crt or key.")
			} else {
				isValidCert = true
			}
		} else if strings.HasSuffix(certPath, ".pem") {
			eUtils.LogInfo(driverConfig.CoreConfig, "Inspecting pem: "+certPath+".")
			pemBlock, _ := pem.Decode(cert)
			if pemBlock == nil {
				eUtils.LogInfo(driverConfig.CoreConfig, "failed to verify certificate PEM.")
			} else {
				isValidCert = true
			}
		} else if strings.HasSuffix(certPath, ".jks") {
			isValidCert = true
		}
		if isValidCert {
			eUtils.LogInfo(driverConfig.CoreConfig, "Certificate passed validation: "+certPath+".")
			certBase64 := base64.StdEncoding.EncodeToString(cert)
			if _, ok := entry.data["certData"]; ok {
				// insecure value entry.
				entry.data["certData"] = certBase64
				eUtils.LogInfo(driverConfig.CoreConfig, "Writing certificate to vault at: "+entry.path+".")
				mod2 := WriteData(driverConfig, entry.path, entry.data, mod)
				if mod != mod2 {
					mod.Stale = true
					mod.Release()
					defer mod2.Release()
					mod = mod2
				}

				certPathSplit := strings.Split(certPath, "/")
				for _, path := range driverConfig.ServiceFilter {
					if strings.Contains(path, certPathSplit[len(certPathSplit)-1]) {
						commonPath := strings.Replace(strings.TrimSuffix(path, ".mf.tmpl"), coreopts.BuildOptions.GetFolderPrefix(nil)+"_templates", "values", -1)
						entry.data["certData"] = "data"
						mod2 := WriteData(driverConfig, commonPath, entry.data, mod)
						if mod != mod2 {
							mod.Stale = true
							mod.Release()
							defer mod2.Release()
							mod = mod2
						}
					}
				}

				eUtils.LogInfo(driverConfig.CoreConfig, "Public cert updated: "+certPath+".")
			} else {
				entryPathParts := strings.Split(entry.path, "/")
				if len(entryPathParts) == 2 {
					secretPath := "super-secrets/" + entryPathParts[1]
					done := false
					// Look up in private entry.
					for _, secretEntry := range *writeStack {
						if secretPath == secretEntry.path {
							if _, ok := secretEntry.data["certData"]; ok {
								secretEntry.data["certData"] = certBase64
								eUtils.LogInfo(driverConfig.CoreConfig, "Writing certificate to vault at: "+secretEntry.path+".")
								mod2 := WriteData(driverConfig, secretEntry.path, secretEntry.data, mod)
								if mod != mod2 {
									mod.Stale = true
									mod.Release()
									defer mod2.Release()
									mod = mod2
								}

								mod2 = WriteData(driverConfig, entry.path, entry.data, mod)
								if mod != mod2 {
									mod.Stale = true
									mod.Release()
									defer mod2.Release()
									mod = mod2
								}

								done = true
								return
							}
						}
					}
					eUtils.LogInfo(driverConfig.CoreConfig, "Cert loaded from: "+certPath+".")

					if done {
						return
					}
				}
			}
		} else {
			if certValidationErr == nil {
				eUtils.LogInfo(driverConfig.CoreConfig, "Cert validation failure. Cert will not be loaded.")
			} else {
				eUtils.LogInfo(driverConfig.CoreConfig, "Cert validation failure.  Cert will not be loaded."+certValidationErr.Error())
			}
			delete(entry.data, "certData")
			delete(entry.data, "certHost")
			delete(entry.data, "certSourcePath")
			delete(entry.data, "certDestPath")
			return
		}
	}
}

// SeedVaultFromData takes file bytes and seeds the vault with contained data
func SeedVaultFromData(driverConfig *config.DriverConfig, filepath string, fData []byte) error {
	driverConfig.CoreConfig.Log.SetPrefix("[SEED]")
	driverConfig.CoreConfig.Log.Println("=========New File==========")
	var verificationData map[interface{}]interface{} // Create a reference for verification. Can't run until other secrets written
	// Unmarshal
	var rawYaml interface{}
	hasEmptyValues := bytes.Contains(fData, []byte("<Enter Secret Here>"))
	isIndexData := strings.HasPrefix(filepath, "Index/") || strings.Contains(filepath, "/PublicIndex/")
	if hasEmptyValues && !isIndexData {
		return eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Incomplete configuration of seed data.  Found default secret data: '<Enter Secret Here>'.  Refusing to continue.", 1)
	}

	if strings.HasPrefix(filepath, "Restricted/") || strings.HasPrefix(filepath, "Protected/") { //Fix incoming pathing for restricted projects
		i := strings.LastIndex(filepath, "/"+driverConfig.CoreConfig.Env)
		if i > 0 {
			filepath = filepath[:i]
		}
	}

	if strings.HasPrefix(filepath, "PublicIndex/") { //Fix incoming pathing for restricted projects
		filepath = "/" + filepath
	}

	err := yaml.Unmarshal(fData, &rawYaml)
	if err != nil {
		return eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, err, 1)
	}

	seed, ok := rawYaml.(map[interface{}]interface{})
	if !ok {
		return eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Invalid yaml file.  Refusing to continue.", 1)
	}

	mapStack := []seedCollection{{"", seed}} // Begin with root of yaml file
	writeStack := make([]writeCollection, 0) // List of all values to write to the vault with p

	// While the stack is not empty
	for len(mapStack) > 0 {
		current := mapStack[0]
		mapStack = mapStack[1:] // Pop the top value

		writeVals := writeCollection{path: current.path, data: map[string]interface{}{}}
		hasLeafNodes := false // Flag to signify this map had values to write

		// Convert nested maps into vault writable data
		for k, v := range current.data {
			if v == nil { // Don't write empty valus, Vault does not handle them
				if !driverConfig.CoreConfig.WantCerts {
					driverConfig.CoreConfig.Log.Printf("Key with no value will not be written: %s\n", current.path+": "+k.(string))
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
	if len(writeStack) == 0 {
		return nil // Nothing to write.
	}

	tokenName := fmt.Sprintf("config_token_%s_unrestricted", driverConfig.CoreConfig.EnvBasis)
	if driverConfig.CoreConfig.CurrentTokenNamePtr != nil &&
		driverConfig.CoreConfig.TokenCache.GetToken(*driverConfig.CoreConfig.CurrentTokenNamePtr) != nil {
		tokenName = *driverConfig.CoreConfig.CurrentTokenNamePtr
	}
	mod, err := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, driverConfig.CoreConfig.Env, true) // Connect to vault
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		mod, err = helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, driverConfig.CoreConfig.Env, false) // Connect to vault
		if mod != nil {
			defer mod.Release()
		}
		if err != nil {
			return eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, err, 1)
		}
	}

	mod.Env = strings.Split(driverConfig.CoreConfig.Env, "_")[0]
	if strings.Contains(filepath, "/PublicIndex/") {
		driverConfig.CoreConfig.Log.Println("Seeding configuration data for the following templates: DataStatistics")
	} else if isIndexData || strings.HasPrefix(filepath, "Restricted/") || strings.HasPrefix(filepath, "Protected/") { //Sets restricted to indexpath due to forward logic using indexpath
		mod.SectionPath = strings.TrimSuffix(filepath, "_seed.yml")
		if len(driverConfig.ServiceFilter) > 0 && isIndexData && !strings.Contains(mod.SectionPath, driverConfig.ServiceFilter[0]) {
			mod.SectionPath = mod.SectionPath[:strings.LastIndex(mod.SectionPath, "/")+1] + driverConfig.ServiceFilter[0] + mod.SectionPath[strings.LastIndex(mod.SectionPath, "/"):]
		}
		//driverConfig.CoreConfig.Log.Println("Seeding configuration data for the following templates:" + driverConfig.ServiceFilter[0])
	} else {
		driverConfig.CoreConfig.Log.Println("Seeding configuration data for the following templates:" + filepath)
	}
	// Write values to vault
	driverConfig.CoreConfig.Log.Println("Please verify that these templates exist in each service")

	for _, entry := range writeStack {
		seedCert := false
		// Output data being written
		// Write data and output any errors
		_, isCertData := entry.data["certData"]

		seedData := !driverConfig.CoreConfig.WantCerts && !isCertData
		if isCertData && strings.HasPrefix(entry.path, "templates/") {
			seedData = true
		}

		if strings.HasPrefix(entry.path, "values/") {
			_, isCertPath := entry.data["certSourcePath"]
			seedCert = (isCertPath || isCertData) && driverConfig.CoreConfig.WantCerts
		}

		// Write Secrets...
		if seedCert {
			sectionPathTemp := mod.SectionPath
			mod.SectionPath = ""
			seedVaultWithCertsFromEntry(driverConfig, mod, &writeStack, &entry)
			mod.SectionPath = sectionPathTemp
		} else if seedData {
			// TODO: Support all services, so range over ServicesWanted....
			// Populate as a slice...
			if driverConfig.ServicesWanted[0] != "" {
				if strings.HasSuffix(entry.path, driverConfig.ServicesWanted[0]) || strings.Contains(entry.path, "Common") {
					mod2 := WriteData(driverConfig, entry.path, entry.data, mod)
					if mod != mod2 {
						mod.Stale = true
						mod.Release()
						defer mod2.Release()
						mod = mod2
					}
				}
			} else if strings.Contains(filepath, "/PublicIndex/") {
				if !strings.Contains(entry.path, "templates") {
					if strings.HasSuffix(filepath, "_seed.yml") {
						filepath = strings.ReplaceAll(filepath, "_seed.yml", "")
					}
					if !strings.HasPrefix(filepath, "super-secrets") {
						filepath = "super-secrets" + filepath
					}

					mod2 := WriteData(driverConfig, filepath, entry.data, mod)
					if mod != mod2 {
						mod.Stale = true
						mod.Release()
						defer mod2.Release()
						mod = mod2
					}
				}
			} else {
				mod2 := WriteData(driverConfig, entry.path, entry.data, mod)
				if mod != mod2 {
					mod.Stale = true
					mod.Release()
					defer mod2.Release()
					mod = mod2
				}
			}
		} else {
			driverConfig.CoreConfig.Log.Printf("\nSkipping non-matching seed data: " + entry.path)
		}
	}

	// Run verification after seeds have been written
	warn, err := verify(driverConfig.CoreConfig, mod, verificationData)
	eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
	eUtils.LogWarningsObject(driverConfig.CoreConfig, warn, false)
	eUtils.LogInfo(driverConfig.CoreConfig, "\nInitialization complete for "+mod.Env+".\n")
	return nil
}

// WriteData takes entry path and date from each iteration of writeStack in SeedVaultFromData and writes to vault
func WriteData(driverConfig *config.DriverConfig, path string, data map[string]interface{}, mod *helperkv.Modifier) *helperkv.Modifier {
	root := strings.Split(path, "/")[0]
	if templateWritten == nil {
		templateWritten = make(map[string]bool)
	}
	if root == "templates" { //Check if templates have already been written in this init call.
		_, ok := templateWritten[path]
		if !ok {
			templateWritten[path] = true
		} else {
			return mod
		}
	}
	tokenName := fmt.Sprintf("config_token_%s_unrestricted", driverConfig.CoreConfig.EnvBasis)
	if driverConfig.CoreConfig.CurrentTokenNamePtr != nil &&
		driverConfig.CoreConfig.TokenCache.GetToken(*driverConfig.CoreConfig.CurrentTokenNamePtr) != nil {
		tokenName = *driverConfig.CoreConfig.CurrentTokenNamePtr
	}
	warn, err := mod.Write(path, data, driverConfig.CoreConfig.Log)
	if err != nil {
		mod, err = helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, driverConfig.CoreConfig.Env, true) // Connect to vault
		if err != nil {
			mod, err = helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, driverConfig.CoreConfig.Env, false) // Connect to vault
			if err != nil {
				// Panic scenario...  Can't reach secrets engine
				eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, err, 1)
			}
			warn, err = mod.Write(path, data, driverConfig.CoreConfig.Log)
			if err != nil {
				// Panic scenario...  Can't reach secrets engine
				eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, err, 1)
			}
		}
	}

	eUtils.LogWarningsObject(driverConfig.CoreConfig, warn, false)
	eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
	// Update value metrics to reflect credential use
	if root == "templates" {
		//Printing out path of each entry so that users can verify that folder structure in seed files are correct
		driverConfig.CoreConfig.Log.Println(coreopts.BuildOptions.GetFolderPrefix(nil) + "_" + path + ".*.tmpl")
		mod.AdjustValue("value-metrics/credentials", data, 1, driverConfig.CoreConfig.Log)
	}
	return mod
}
