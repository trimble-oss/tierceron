package zccommon

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func ConfigCertLibHelper(token string,
	address string,
	env string,
	templatePath string,
	configuredFilePath string,
	project string,
	service string,
	wantCerts bool,
) (string, string, error) {
	logger := log.New(os.Stdout, "[configCertLibHelper]", log.LstdFlags)
	mod, err := helperkv.NewModifier(false, &token, &address, env, nil, true, logger)
	if err != nil {
		return "", "", err
	}
	defer mod.Release()
	tokenCache, err := cache.NewTokenCache(fmt.Sprintf("config_token_%s", env), &token, &address)
	if err != nil {
		return "", "", err
	}
	driverConfig := &config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			WantCerts:  wantCerts,
			TokenCache: tokenCache,
			Insecure:   true,
			Log:        logger,
		},

		ZeroConfig: true,
		StartDir:   append([]string{}, "trc_templates"),
	}
	mod.Env = env
	serviceParts := strings.Split(service, ".")
	configTemplate, configuredCert, _, err := vcutils.ConfigTemplate(driverConfig, mod, templatePath, true, project, serviceParts[0], wantCerts, true)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
	}

	if wantCerts {
		return "", base64.StdEncoding.EncodeToString([]byte(configuredCert[1])), err
	} else {
		return configTemplate, "", err
	}
}
