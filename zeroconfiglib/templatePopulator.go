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
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
)

func configCertLibHelper(token string,
	address string,
	env string,
	templatePath string,
	configuredFilePath string,
	project string,
	service string,
	wantCerts bool) (string, string, error) {
	logger := log.New(os.Stdout, "[configCertLibHelper]", log.LstdFlags)
	mod, err := helperkv.NewModifier(false, &token, &address, env, nil, true, logger)
	mod.Env = env
	driverConfig := &config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			WantCerts:  wantCerts,
			TokenCache: cache.NewTokenCache(fmt.Sprintf("config_token_%s", env), &token),
			Insecure:   true,
			Log:        logger,
		},

		ZeroConfig: true,
		StartDir:   append([]string{}, "trc_templates"),
	}
	if err != nil {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, err.Error(), false)
		return "", "", err
	}
	serviceParts := strings.Split(service, ".")
	configTemplate, configuredCert, _, err := vcutils.ConfigTemplate(driverConfig, mod, templatePath, true, project, serviceParts[0], wantCerts, true)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
	}

	mod.Close()

	if wantCerts {
		return "", base64.StdEncoding.EncodeToString([]byte(configuredCert[1])), err
	} else {
		return configTemplate, "", err
	}
}

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)
	logger.Println("NCLib Version: " + "1.21")
	configuredTemplate, _, err := configCertLibHelper(token,
		address,
		env,
		templatePath,
		configuredFilePath,
		project,
		service,
		false)
	if err != nil {
		return C.CString("")
	}

	return C.CString(configuredTemplate)
}

//export ConfigCertLib
func ConfigCertLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigCertLib]", log.LstdFlags)
	logger.Println("NCLib Version: " + "1.21")
	certBase64, _, err := configCertLibHelper(token,
		address,
		env,
		templatePath,
		configuredFilePath,
		project,
		service,
		true)
	if err != nil {
		return C.CString("")
	}

	return C.CString(certBase64)
}

func main() {}
