package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	il "tierceron/trcinit/initlib"
	"tierceron/utils"
	"tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"

	configcore "VaultConfig.Bootstrap/configcore"
)

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func main() {
	fmt.Println("Version: " + "1.3")
	dirPtr := flag.String("dir", "trc_templates", "Directory containing template files for vault")
	envPtr := flag.String("env", "dev", "Environement in vault")
	addrPtr := flag.String("addr", configcore.VaultHostPort, "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	secretIDPtr := flag.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this trcpub to access the vault")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")

	logBuffer := new(bytes.Buffer)
	logger := log.New(logBuffer, "[INIT]", log.LstdFlags)

	flag.Parse()

	if len(*tokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			utils.CheckError(fmt.Errorf("Need both public and secret app role to retrieve token from vault"), true)
		}
		v, err := sys.NewVault(*insecurePtr, *addrPtr, *envPtr, false, *pingPtr)
		utils.CheckError(err, true)

		master, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
		utils.CheckError(err, true)

		mod, err := kv.NewModifier(*insecurePtr, master, *addrPtr, *envPtr, nil)
		utils.CheckError(err, true)
		mod.Env = "bamboo"

		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		utils.CheckError(err, true)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = utils.LoginToLocal()
		fmt.Println(*envPtr)
		utils.CheckError(err, true)
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
	fmt.Printf("Uploading templates in %s to vault\n", *dirPtr)

	mod, err := kv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil)
	utils.CheckError(err, true)

	envSlice := make([]string, 0)
	if strings.Contains(*envPtr, "*") {
		//Ask vault for list of dev.* environments, add to envSlice
		mod.Env = strings.Split(*envPtr, "*")[0]
		listValues, err := mod.ListEnv("values/")
		if err != nil {
			logger.Printf(err.Error())
		}

		if listValues == nil {
			fmt.Println("No enterprise IDs were found.")
			os.Exit(1)
		}
		for _, valuesPath := range listValues.Data {
			for _, envInterface := range valuesPath.([]interface{}) {
				env := envInterface.(string)
				if strings.Contains(env, ".") && strings.Contains(env, mod.Env) {
					env = strings.ReplaceAll(env, "/", "")
					envSlice = append(envSlice, env)
				}
			}
		}
	} else {
		envSlice = append(envSlice, *envPtr)
	}

	for _, env := range envSlice {
		mod.Env = env
		err, warn := il.UploadTemplateDirectory(mod, *dirPtr, logger)

		if err != nil {
			if strings.Contains(err.Error(), "x509: certificate") {
				os.Exit(-1)
			}
		}

		utils.CheckError(err, true)
		utils.CheckWarnings(warn, true)
	}
}

func uploadTemplates(insecure bool, addr string, token string, dirName string, env string, logger *log.Logger) {
	logger.Println("dirName")
	logger.Println(dirName)
	// Open directory
	files, err := ioutil.ReadDir(dirName)
	utils.CheckError(err, true)

	// Use name of containing directory as the template subdirectory
	splitDir := strings.SplitAfterN(dirName, "/", 2)
	subDir := splitDir[len(splitDir)-1]
	logger.Println("subDir")
	logger.Println(subDir)

	// Create modifier
	mod, err := kv.NewModifier(insecure, token, addr, env, nil)
	utils.CheckError(err, true)
	mod.Env = env

	// Parse through files
	for _, file := range files {
		if file.IsDir() { // Recurse folders
			uploadTemplates(insecure, addr, token, dirName+"/"+file.Name(), env, logger)
			// if err != nil || len(warn) > 0 {
			// 	return err, warn
			// }
			continue
		}
		// Extract extension and name
		ext := filepath.Ext(file.Name())
		name := file.Name()
		name = name[0 : len(name)-len(ext)] // Truncate extension

		if ext == ".tmpl" { // Only upload template files
			fmt.Printf("Found template file %s for %s\n", file.Name(), mod.Env)
			logger.Println("Found template file %s for %s\n", file.Name(), mod.Env)
			// Seperate name and extension one more time for saving to vault
			ext = filepath.Ext(name)
			name = name[0 : len(name)-len(ext)]

			// Extract values
			extractedValues, err := utils.Parse(dirName+"/"+file.Name(), subDir, name)
			utils.CheckError(err, true)

			// Open file
			f, err := os.Open(dirName + "/" + file.Name())
			utils.CheckError(err, true)

			// Read the file
			fileBytes := make([]byte, file.Size())
			_, err = f.Read(fileBytes)
			utils.CheckError(err, true)

			// Construct template path for vault
			templatePath := "templates/" + subDir + "/" + name + "/template-file"
			logger.Println("\tUploading template to path:\t%s\n", templatePath)

			// Construct value path for vault
			valuePath := "values/" + subDir + "/" + name
			logger.Println("\tUploading values to path:\t%s\n", valuePath)

			// Write templates to vault and output errors/warnings
			warn, err := mod.Write(templatePath, map[string]interface{}{"data": fileBytes, "ext": ext})
			utils.CheckError(err, false)
			utils.CheckWarnings(warn, false)

			// Write values to vault and output any errors/warnings
			warn, err = mod.Write(valuePath, extractedValues)
			utils.CheckError(err, false)
			utils.CheckWarnings(warn, false)
		}
	}
}
