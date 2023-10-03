package initlib

import (
	"os"
	"path/filepath"

	eUtils "github.com/trimble-oss/tierceron/utils"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
)

// UploadPolicies accepts a file directory and vault object to upload policies to. Logs to pased logger
func UploadPolicies(config *eUtils.DriverConfig, dir string, v *sys.Vault, noPermissions bool) error {
	config.Log.SetPrefix("[POLICY]")
	config.Log.Printf("Writing policies from %s\n", dir)
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	eUtils.LogErrorObject(config, err, true)
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		if ext == ".hcl" { // Write policy to vault
			config.Log.Printf("\tFound policy file: %s\n", file.Name())
			if noPermissions {
				err = v.CreateEmptyPolicy(filename)
			} else {
				err = v.CreatePolicyFromFile(filename, dir+"/"+file.Name())
			}
			eUtils.LogErrorObject(config, err, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetExistsPolicies accepts a file directory and vault object to check policies for. Logs to pased logger
func GetExistsPolicies(config *eUtils.DriverConfig, dir string, v *sys.Vault) (bool, error) {
	config.Log.SetPrefix("[POLICY]")
	config.Log.Printf("Checking exists token policies from %s\n", dir)
	files, err := os.ReadDir(dir)
	if err != nil {
		return false, nil
	}

	allExists := false

	eUtils.LogErrorObject(config, err, true)
	for _, file := range files {
		// Extract and truncate file name
		config.Log.Printf("\tFound token policy file: %s\n", file.Name())
		exists, err := v.GetExistsPolicyFromFileName(file.Name())
		eUtils.LogErrorObject(config, err, false)
		if err != nil {
			return false, err
		}
		allExists = allExists || exists
	}

	return allExists, nil
}
