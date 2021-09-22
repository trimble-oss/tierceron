package initlib

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"tierceron/utils"
	sys "tierceron/vaulthelper/system"
)

//UploadTokenCidrRoles accepts a file directory and vault object to upload token roles to. Logs to pased logger
func UploadTokenCidrRoles(dir string, v *sys.Vault, logger *log.Logger) error {
	logger.SetPrefix("[ROLE]")
	logger.Printf("Writing token roles from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	utils.LogErrorObject(err, logger, true)
	if err != nil {
		return err
	}
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		logger.Printf("\tFound token role file: %s\n", file.Name())
		err = v.CreateTokenCidrRoleFromFile(dir + "/" + file.Name())
		utils.LogErrorObject(err, logger, false)
		if err != nil {
			return err
		}
	}
	return nil
}

//GetExistsRole accepts a file directory and vault object to check existence of token roles. Logs to pased logger
func GetExistsRoles(dir string, v *sys.Vault, logger *log.Logger) (bool, error) {
	logger.SetPrefix("[ROLE]")
	logger.Printf("Checking exists token roles from %s\n", dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, nil
	}

	allExists := false

	utils.LogErrorObject(err, logger, true)
	for _, file := range files {
		// Extract and truncate file name
		logger.Printf("\tFound token role file: %s\n", file.Name())
		exists, err := v.GetExistsTokenRoleFromFile(dir + "/" + file.Name())
		utils.LogErrorObject(err, logger, false)
		if err != nil {
			return false, err
		}
		allExists = allExists || exists
	}

	return allExists, nil
}
