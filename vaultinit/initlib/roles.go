package initlib

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"bitbucket.org/dexterchaney/whoville/utils"
	sys "bitbucket.org/dexterchaney/whoville/vaulthelper/system"
)

//UploadTokenCidrRoles accepts a file directory and vault object to upload token roles to. Logs to pased logger
func UploadTokenCidrRoles(dir string, v *sys.Vault, logger *log.Logger) {
	logger.SetPrefix("[ROLE]")
	logger.Printf("Writing token roles from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	utils.LogErrorObject(err, logger, true)
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		logger.Printf("\tFound token role file: %s\n", file.Name())
		err = v.CreateTokenCidrRoleFromFile(dir + "/" + file.Name())
		utils.LogErrorObject(err, logger, false)
	}
}
