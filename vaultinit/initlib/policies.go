package initlib

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"bitbucket.org/dexterchaney/whoville/utils"
	sys "bitbucket.org/dexterchaney/whoville/vaulthelper/system"
)

//UploadPolicies accepts a file directory and vault object to upload policies to. Logs to pased logger
func UploadPolicies(dir string, v *sys.Vault, noPermissions bool, logger *log.Logger) {
	logger.SetPrefix("[POLICY]")
	logger.Printf("Writing policies from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	utils.LogErrorObject(err, logger, true)
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		if ext == ".hcl" { // Write policy to vault
			logger.Printf("\tFound policy file: %s\n", file.Name())
			if noPermissions {
				err = v.CreateEmptyPolicy(filename)
			} else {
				err = v.CreatePolicyFromFile(filename, dir+"/"+file.Name())
			}
			utils.LogErrorObject(err, logger, false)
		}

	}
}
