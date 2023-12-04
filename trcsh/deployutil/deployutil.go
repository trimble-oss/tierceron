package deployutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/trimble-oss/tierceron/capauth"
	vcutils "github.com/trimble-oss/tierceron/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

// Loads a plugin's deploy.trc script directly from vault.
func LoadPluginDeploymentScript(config *eUtils.DriverConfig, trcshConfig *capauth.TrcShConfig, pwd string) ([]byte, error) {
	if strings.Contains(pwd, "TrcDeploy") && len(config.DeploymentConfig) > 0 {
		if deployment, ok := config.DeploymentConfig["trcplugin"]; ok {
			mergedEnvRaw := config.EnvRaw
			// Swapping in project root...
			configRoleSlice := strings.Split(*trcshConfig.ConfigRole, ":")
			tokenName := "config_token_" + config.EnvRaw
			readToken := ""
			autoErr := eUtils.AutoAuth(config, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &config.Env, &config.VaultAddress, &mergedEnvRaw, "config.yml", false)
			if autoErr != nil {
				fmt.Println("Missing auth components.")
				return nil, autoErr
			}

			mod, err := helperkv.NewModifier(config.Insecure, readToken, *trcshConfig.VaultAddress, config.EnvRaw, config.Regions, true, config.Log)
			if err != nil {
				fmt.Println("Unable to obtain resources for deployment")
				return nil, err
			}
			mod.Env = config.EnvRaw
			deploymentConfig, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			if err != nil {
				fmt.Println("Unable to obtain config for deployment")
				return nil, err
			}
			deploymentConfig["trcpluginalias"] = deployment
			config.DeploymentConfig = deploymentConfig
			if trcDeployRoot, ok := config.DeploymentConfig["trcdeployroot"]; ok {
				config.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
				config.EndDir = trcDeployRoot.(string)
			}

			if trcProjectService, ok := config.DeploymentConfig["trcprojectservice"]; ok && strings.Contains(trcProjectService.(string), "/") {
				var content []byte
				trcProjectServiceSlice := strings.Split(trcProjectService.(string), "/")
				config.ZeroConfig = true
				contentArray, _, _, err := vcutils.ConfigTemplate(config, mod, fmt.Sprintf("./trc_templates/%s/deploy/deploy.trc.tmpl", trcProjectService.(string)), true, trcProjectServiceSlice[0], trcProjectServiceSlice[1], false, true)
				config.ZeroConfig = false
				if err != nil {
					eUtils.LogErrorObject(config, err, false)
					return nil, err
				}
				content = []byte(contentArray)
				return content, nil
			} else {
				fmt.Println("Project not configured and ready for deployment.  Missing projectservice")
				return nil, errors.New("project not configured and ready for deployment.  missing projectservice")
			}
		}
	}
	return nil, errors.New("not deployer")
}
