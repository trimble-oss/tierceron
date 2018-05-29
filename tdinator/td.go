package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"vault-helper/kv"
)

// Reads in template files in specified directory
// Template directory should contain directories for each service
// Templates are uploaded to templates/<service>/<file name>/template-file
// The file is saved under the data key, and the extension under the ext key
// Vault automatically encodes the file into base64

func main() {
	dirPtr := flag.String("dir", "seeds", "Directory containing template files for vault")
	addrPtr := flag.String("addr", "http://127.0.0.1:8200", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")

	flag.Parse()
	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
	fmt.Printf("Uploading templates in %s to vault\n", *dirPtr)

	dirs, err := ioutil.ReadDir(*dirPtr)
	utils.CheckError(err)

	// Parse each subdirectory as a service name
	for _, dir := range dirs {
		if dir.IsDir() {
			pathName := *dirPtr + "/" + dir.Name()
			uploadTemplates(*addrPtr, *tokenPtr, pathName)
		}
	}
}

func uploadTemplates(addr string, token string, dirName string) {
	// Open directory
	files, err := ioutil.ReadDir(dirName)
	utils.CheckError(err)

	// Use name of containing directory as the template subdirectory
	splitDir := strings.SplitAfter(dirName, "/")
	subDir := splitDir[len(splitDir)-1]

	// Create modifier
	mod, err := kv.NewModifier(token, addr)
	utils.CheckError(err)

	// Parse through files
	for _, file := range files {
		// Extract extension and name
		ext := filepath.Ext(file.Name())
		name := file.Name()
		name = name[0 : len(name)-len(ext)] // Truncate extension

		if ext == ".tmpl" { // Only upload template files
			fmt.Printf("Found template file %s\n", file.Name())

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

			// Construct path for writing to vault
			vaultPath := "templates/" + subDir + "/" + name + "/template-file"
			fmt.Printf("\tUploading template to path:   %s\n", vaultPath)

			// Write to vault and output any errors
			warn, err := mod.Write(vaultPath, map[string]interface{}{"data": fileBytes, "ext": ext})
			if len(warn) > 0 {
				fmt.Printf("\tWarnings %v\n", warn)
			}
			utils.CheckError(err)
		}
	}
}
