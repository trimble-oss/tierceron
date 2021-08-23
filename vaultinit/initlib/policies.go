package initlib

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"Vault.Whoville/utils"
	sys "Vault.Whoville/vaulthelper/system"
)

//UploadPolicies accepts a file directory and vault object to upload policies to. Logs to pased logger
func UploadPolicies(dir string, v *sys.Vault, noPermissions bool, logger *log.Logger) error {
	logger.SetPrefix("[POLICY]")
	logger.Printf("Writing policies from %s\n", dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

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
			if err != nil {
				return err
			}
		}
	}
	return nil
}

//GetExistsPolicies accepts a file directory and vault object to check policies for. Logs to pased logger
func GetExistsPolicies(dir string, v *sys.Vault, logger *log.Logger) (bool, error) {
	logger.SetPrefix("[POLICY]")
	logger.Printf("Checking exists token policies from %s\n", dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, nil
	}

	allExists := false

	utils.LogErrorObject(err, logger, true)
	for _, file := range files {
		// Extract and truncate file name
		logger.Printf("\tFound token policy file: %s\n", file.Name())
		exists, err := v.GetExistsPolicyFromFileName(file.Name())
		utils.LogErrorObject(err, logger, false)
		if err != nil {
			return false, err
		}
		allExists = allExists || exists
	}

	return allExists, nil
}
