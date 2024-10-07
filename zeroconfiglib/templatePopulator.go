package main

import (
	"C"

	"encoding/base64"

	"github.com/trimble-oss/tierceron/pkg/utils/config"

	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)
import (
	"log"
	"os"

	"github.com/trimble-oss/tierceron/pkg/core"
)

//export ConfigTemplateLib
func ConfigTemplateLib(tokenPtr *string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)

	logger.Println("NCLib Version: " + "1.20")
	mod, err := helperkv.NewModifier(false, tokenPtr, &address, env, nil, true, logger)
	mod.Env = env
	driverConfig := &config.DriverConfig{
		CoreConfig: core.CoreConfig{
			WantCerts: false,
			Insecure:  false,
			Log:       logger,
		},
		ZeroConfig: true,
		StartDir:   append([]string{}, "trc_templates"),
	}

	if err != nil {
		eUtils.LogErrorObject(&driverConfig.CoreConfig, err, false)
	}

	configuredTemplate, _, _, err := vcutils.ConfigTemplate(driverConfig, mod, templatePath, true, project, service, false, true)
	if err != nil {
		eUtils.LogErrorObject(&driverConfig.CoreConfig, err, false)
	}

	mod.Close()

	return C.CString(configuredTemplate)
}

//export ConfigCertLib
func ConfigCertLib(tokenPtr *string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)
	logger.Println("NCLib Version: " + "1.20")
	mod, err := helperkv.NewModifier(false, tokenPtr, &address, env, nil, true, logger)
	mod.Env = env
	driverConfig := &config.DriverConfig{
		CoreConfig: core.CoreConfig{
			WantCerts: true,
			Insecure:  false,
			Log:       logger,
		},

		ZeroConfig: true,
		StartDir:   append([]string{}, "trc_templates"),
	}
	if err != nil {
		eUtils.LogErrorMessage(&driverConfig.CoreConfig, err.Error(), false)
		return C.CString("")
	}
	_, configuredCert, _, err := vcutils.ConfigTemplate(driverConfig, mod, templatePath, true, project, service, true, true)
	if err != nil {
		eUtils.LogErrorObject(&driverConfig.CoreConfig, err, false)
	}

	mod.Close()

	certBase64 := base64.StdEncoding.EncodeToString([]byte(configuredCert[1]))

	return C.CString(certBase64)
}

func main() {}
