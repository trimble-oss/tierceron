package initlib

import (
	"io/ioutil"
	"path/filepath"

	eUtils "github.com/trimble-oss/tierceron/utils"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
)

// UploadTokenCidrRoles accepts a file directory and vault object to upload token roles to. Logs to pased logger
func UploadTokenCidrRoles(config *eUtils.DriverConfig, dir string, v *sys.Vault) error {
	config.Log.SetPrefix("[ROLE]")
	config.Log.Printf("Writing token roles from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	eUtils.LogErrorObject(config, err, false)
	if err != nil {
		return err
	}
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		config.Log.Printf("\tFound token role file: %s\n", file.Name())
		err = v.CreateTokenCidrRoleFromFile(dir + "/" + file.Name())
		eUtils.LogErrorObject(config, err, false)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetExistsRole accepts a file directory and vault object to check existence of token roles. Logs to pased logger
func GetExistsRoles(config *eUtils.DriverConfig, dir string, v *sys.Vault) (bool, error) {
	config.Log.SetPrefix("[ROLE]")
	config.Log.Printf("Checking exists token roles from %s\n", dir)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return false, nil
	}

	allExists := false

	eUtils.LogErrorObject(config, err, true)
	for _, file := range files {
		// Extract and truncate file name
		config.Log.Printf("\tFound token role file: %s\n", file.Name())
		exists, err := v.GetExistsTokenRoleFromFile(dir + "/" + file.Name())
		eUtils.LogErrorObject(config, err, false)
		if err != nil {
			return false, err
		}
		allExists = allExists || exists
	}

	return allExists, nil
}
