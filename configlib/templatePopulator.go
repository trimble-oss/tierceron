package main

import (
	"C"

	"encoding/base64"

	vcutils "github.com/trimble-oss/tierceron/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)
import (
	"log"
	"os"
)

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)

	logger.Println("NCLib Version: " + "1.12")
	mod, err := helperkv.NewModifier(false, token, address, env, nil, true, logger)
	mod.Env = env
	config := &eUtils.DriverConfig{
		Insecure: false,
		Log:      logger,
	}

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}

	configuredTemplate, _, _, err := vcutils.ConfigTemplate(config, mod, templatePath, true, project, service, false, true)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}

	mod.Close()

	return C.CString(configuredTemplate)
}

//export ConfigCertLib
func ConfigCertLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)
	logger.Println("NCLib Version: " + "1.12")
	mod, err := helperkv.NewModifier(false, token, address, env, nil, true, logger)
	mod.Env = env
	config := &eUtils.DriverConfig{
		Insecure: false,
		Log:      logger,
	}
	if err != nil {
		eUtils.LogErrorMessage(config, err.Error(), false)
		return C.CString("")
	}
	_, configuredCert, _, err := vcutils.ConfigTemplate(config, mod, templatePath, true, project, service, true, true)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}

	mod.Close()

	certBase64 := base64.StdEncoding.EncodeToString([]byte(configuredCert[1]))

	return C.CString(certBase64)
}

func main() {}
