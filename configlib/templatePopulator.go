package main

import (
	"C"

	"encoding/base64"

	"tierceron/trcconfig/utils"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"
)
import (
	"log"
	"os"
)

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)

	logger.Println("NCLib Version: " + "1.12")
	mod, err := kv.NewModifier(false, token, address, env, nil, logger)
	mod.Env = env
	config := &eUtils.DriverConfig{
		Insecure: false,
		Log:      logger,
	}

	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}

	configuredTemplate, _, _, err := utils.ConfigTemplate(config, mod, templatePath, true, project, service, false, true)

	mod.Close()

	return C.CString(configuredTemplate)
}

//export ConfigCertLib
func ConfigCertLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)
	logger.Println("NCLib Version: " + "1.12")
	mod, err := kv.NewModifier(false, token, address, env, nil, logger)
	mod.Env = env
	if err != nil {
		panic(err)
	}
	config := &eUtils.DriverConfig{
		Insecure: false,
		Log:      logger,
	}
	_, configuredCert, _, err := utils.ConfigTemplate(config, mod, templatePath, true, project, service, true, true)

	mod.Close()

	certBase64 := base64.StdEncoding.EncodeToString([]byte(configuredCert[1]))

	return C.CString(certBase64)
}

func main() {}
