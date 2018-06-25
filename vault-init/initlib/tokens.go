package initlib

import (
	"bitbucket.org/dexterchaney/whoville/utils"
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
)

//UploadTokens accepts a file directory and vault object to upload tokens to. Logs to pased logger
func UploadTokens(dir string, v *sys.Vault, logger *log.Logger) {
	logger.SetPrefix("[TOKEN]")
	logger.Printf("Writing tokens from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	utils.LogErrorObject(err, logger)
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		if ext == ".yml" || ext == ".yaml" { // Request token from vault
			logger.Printf("\tFound token file: %s\n", file.Name())
			tokenName, err := v.CreateTokenFromFile(dir + "/" + file.Name())
			utils.LogErrorObject(err, logger)
			fmt.Printf("Created token %-30s %s\n", filename+":", tokenName)
		}

	}
}
