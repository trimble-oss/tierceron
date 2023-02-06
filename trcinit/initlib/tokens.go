package initlib

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/utils"
	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
	pb "github.com/trimble-oss/tierceron/webapi/rpc/apinator"
)

// UploadTokens accepts a file directory and vault object to upload tokens to. Logs to pased logger
func UploadTokens(config *eUtils.DriverConfig, dir string, fileFilterPtr *string, v *sys.Vault) []*pb.InitResp_Token {
	tokens := []*pb.InitResp_Token{}
	config.Log.SetPrefix("[TOKEN]")
	config.Log.Printf("Writing tokens from %s\n", dir)
	files, err := ioutil.ReadDir(dir)

	eUtils.LogErrorObject(config, err, true)
	for _, file := range files {
		// Extract and truncate file name
		filename := file.Name()
		ext := filepath.Ext(filename)
		filename = filename[0 : len(filename)-len(ext)]

		if ext == ".yml" || ext == ".yaml" { // Request token from vault
			if *fileFilterPtr != "" && !strings.Contains(file.Name(), *fileFilterPtr) {
				continue
			}
			config.Log.Printf("\tFound token file: %s\n", file.Name())
			tokenName, err := v.CreateTokenFromFile(dir + "/" + file.Name())
			eUtils.LogErrorObject(config, err, true)

			if err == nil {
				fmt.Printf("Created token %-30s %s\n", filename+":", tokenName)
				tokens = append(tokens, &pb.InitResp_Token{
					Name:  filename,
					Value: tokenName,
				})
			}
		}

	}
	return tokens
}
