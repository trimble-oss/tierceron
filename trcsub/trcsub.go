package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	il "tierceron/trcinit/initlib"
	"tierceron/utils"
	eUtils "tierceron/utils"
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
	logFilePtr := flag.String("log", "./trcpub.log", "Output path for log files")

	flag.Parse()

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/trcsub.log" {
		*logFilePtr = "./trcsub.log"
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	logger := log.New(f, "[INIT]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}
	eUtils.CheckError(config, err, true)

	if len(*tokenNamePtr) > 0 {
		if len(*appRoleIDPtr) == 0 || len(*secretIDPtr) == 0 {
			utils.CheckError(config, fmt.Errorf("Need both public and secret app role to retrieve token from vault"), true)
		}
		v, err := sys.NewVault(*insecurePtr, *addrPtr, *envPtr, false, *pingPtr, false, logger)
		utils.CheckError(config, err, true)

		master, err := v.AppRoleLogin(*appRoleIDPtr, *secretIDPtr)
		utils.CheckError(config, err, true)

		mod, err := kv.NewModifier(*insecurePtr, master, *addrPtr, *envPtr, nil, logger)
		utils.CheckError(config, err, true)
		mod.Env = "bamboo"

		*tokenPtr, err = mod.ReadValue("super-secrets/tokens", *tokenNamePtr)
		utils.CheckError(config, err, true)
	}

	if len(*envPtr) >= 5 && (*envPtr)[:5] == "local" {
		var err error
		*envPtr, err = utils.LoginToLocal()
		fmt.Println(*envPtr)
		utils.CheckError(config, err, true)
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
	fmt.Printf("Downloading templates from vault to %s\n", *dirPtr)

	mod, err := kv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, logger)
	utils.CheckError(config, err, true)
	mod.Env = *envPtr

	err, warn := il.UploadTemplateDirectory(mod, *dirPtr, logger)
	if err != nil {
		if strings.Contains(err.Error(), "x509: certificate") {
			os.Exit(-1)
		}
	}

	utils.CheckError(config, err, true)
	utils.CheckWarnings(config, warn, true)
}

func downloadTemplates(config *eUtils.DriverConfig, dirName string) {
	config.Log.Printf("dirName: %s\n", dirName)
	// Open directory
	files, err := ioutil.ReadDir(dirName)
	utils.CheckError(config, err, true)

	// Use name of containing directory as the template subdirectory
	splitDir := strings.SplitAfterN(dirName, "/", 2)
	subDir := splitDir[len(splitDir)-1]
	config.Log.Printf("subDir: %s\n", subDir)

	// Create modifier
	mod, err := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.Env, nil, config.Log)
	utils.CheckError(config, err, true)
	mod.Env = config.Env

	// Parse through files
	for _, file := range files {
		if file.IsDir() { // Recurse folders
			downloadTemplates(config, dirName+"/"+file.Name())
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
			fmt.Printf("Found template file %s\n", file.Name())
			config.Log.Println(fmt.Sprintf("Found template file %s", file.Name()))
			// Seperate name and extension one more time for saving to vault
			ext = filepath.Ext(name)
			name = name[0 : len(name)-len(ext)]

			// Extract values
			extractedValues, err := utils.Parse(dirName+"/"+file.Name(), subDir, name)
			utils.CheckError(config, err, true)

			// Open file
			f, err := os.Open(dirName + "/" + file.Name())
			utils.CheckError(config, err, true)

			// Read the file
			fileBytes := make([]byte, file.Size())
			_, err = f.Read(fileBytes)
			utils.CheckError(config, err, true)

			// Construct template path for vault
			templatePath := "templates/" + subDir + "/" + name + "/template-file"
			config.Log.Println("\tUploading template to path:\t%s\n", templatePath)

			// Construct value path for vault
			valuePath := "values/" + subDir + "/" + name
			config.Log.Println("\tUploading values to path:\t%s\n", valuePath)

			// Write templates to vault and output errors/warnings
			warn, err := mod.Write(templatePath, map[string]interface{}{"data": fileBytes, "ext": ext})
			utils.CheckError(config, err, false)
			utils.CheckWarnings(config, warn, false)

			// Write values to vault and output any errors/warnings
			warn, err = mod.Write(valuePath, extractedValues)
			utils.CheckError(config, err, false)
			utils.CheckWarnings(config, warn, false)
		}
	}
}
