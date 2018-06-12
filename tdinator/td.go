package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func main() {
	dirPtr := flag.String("dir", "vault_templates", "Directory containing template files for vault")
	envPtr := flag.String("env", "secret", "Environement in vault")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	certPathPtr := flag.String("certPath", "certs/cert_files/serv_cert.pem", "Path to the server certificate")

	flag.Parse()
	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
	fmt.Printf("Uploading templates in %s to vault\n", *dirPtr)

	dirs, err := ioutil.ReadDir(*dirPtr)
	utils.CheckError(err)

	// Parse each subdirectory as a service name
	for _, dir := range dirs {
		if dir.IsDir() {
			pathName := *dirPtr + "/" + dir.Name()
			uploadTemplates(*addrPtr, *tokenPtr, pathName, *certPathPtr, *envPtr)
		}
	}
}

func uploadTemplates(addr string, token string, dirName string, certPath string, env string) {
	// Open directory
	files, err := ioutil.ReadDir(dirName)
	utils.CheckError(err)

	// Use name of containing directory as the template subdirectory
	splitDir := strings.SplitAfter(dirName, "/")
	subDir := splitDir[len(splitDir)-1]

	// Create modifier
	mod, err := kv.NewModifier(token, addr, certPath)
	utils.CheckError(err)
	mod.Env = env

	// Parse through files
	for _, file := range files {
		// Extract extension and name
		ext := filepath.Ext(file.Name())
		name := file.Name()
		name = name[0 : len(name)-len(ext)] // Truncate extension

		if ext == ".tmpl" { // Only upload template files
			fmt.Printf("Found template file %s\n", file.Name())

			// Extract values
			extractedValues, err := utils.Parse(dirName + "/" + file.Name())
			utils.CheckError(err)
			fmt.Println("\tExtracted values:")
			for k, v := range extractedValues {
				fmt.Printf("\t\t%-30s%v\n", k, v)
			}

			// Open file
			f, err := os.Open(dirName + "/" + file.Name())
			utils.CheckError(err)

			// Read the file
			fileBytes := make([]byte, file.Size())
			_, err = f.Read(fileBytes)
			utils.CheckError(err)

			// Seperate name and extension one more time for saving to vault
			ext = filepath.Ext(name)
			name = name[0 : len(name)-len(ext)]

			// Construct template path for vault
			templatePath := "templates/" + subDir + "/" + name + "/template-file"
			fmt.Printf("\tUploading template to path:\t%s\n", templatePath)

			// Construct value path for vault
			valuePath := "values/" + subDir + "/" + name
			fmt.Printf("\tUploading values to path:\t%s\n", valuePath)

			// Write templates to vault and output errors/warnings
			warn, err := mod.Write(templatePath, map[string]interface{}{"data": fileBytes, "ext": ext})
			if len(warn) > 0 {
				fmt.Printf("\tWarnings %v\n", warn)
			}
			utils.CheckError(err)

			// Write values to vault and output any errors/warnings
			warn, err = mod.Write(valuePath, extractedValues)
			if len(warn) > 0 {
				fmt.Printf("\tWarnings %v\n", warn)
			}
			utils.CheckError(err)
		}
	}
}

// Things to update:
//		Ports?
//		URL references?
// 		Domain headers?
//		Redirects?
