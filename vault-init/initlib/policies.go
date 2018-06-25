package initlib

import (
	"bitbucket.org/dexterchaney/whoville/utils"
	sys "bitbucket.org/dexterchaney/whoville/vault-helper/system"
	"io/ioutil"
	"log"
	"path/filepath"
)

//UploadPolicies accepts a file directory and vault object to upload policies to. Logs to pased logger
func UploadPolicies(dir string, v *sys.Vault, logger *log.Logger) {
	logger.SetPrefix("[POLICY]")
	logger.Printf("Writing policies from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	utils.LogErrorObject(err, logger)
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		if ext == ".hcl" { // Write policy to vault
			logger.Printf("\tFound policy file: %s\n", file.Name())
			err = v.CreatePolicyFromFile(filename, dir+"/"+file.Name())
			utils.LogErrorObject(err, logger)
		}

	}
}
