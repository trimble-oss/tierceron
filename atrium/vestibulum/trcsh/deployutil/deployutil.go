package deployutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

// Loads a plugin's deploy.trc script directly from vault.
func LoadPluginDeploymentScript(config *eUtils.DriverConfig, trcshConfig *capauth.TrcShConfig, pwd string) ([]byte, error) {
	if strings.Contains(pwd, "TrcDeploy") && len(config.DeploymentConfig) > 0 {
		if deployment, ok := config.DeploymentConfig["trcplugin"]; ok {
			if deploymentAlias, deployAliasOk := config.DeploymentConfig["trcpluginalias"]; deployAliasOk {
				deployment = deploymentAlias
			}
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
			tempEnv := config.EnvRaw
			envParts := strings.Split(config.EnvRaw, "-")
			mod.Env = envParts[0]
			fmt.Printf("Loading deployment details for %s and env %s", deployment, mod.Env)
			deploymentConfig, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			mod.Env = tempEnv
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

// Gets list of supported deployers for current environment.
func GetDeployers(config *eUtils.DriverConfig) ([]string, error) {

	// Swapping in project root...
	configRoleSlice := strings.Split(config.AppRoleConfig, ":")
	mergedEnvRaw := config.EnvRaw
	tokenName := "config_token_" + config.Env
	readToken := ""
	autoErr := eUtils.AutoAuth(config, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &config.Env, &config.VaultAddress, &mergedEnvRaw, "config.yml", false)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return nil, autoErr
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, &readToken)
	}

	mod, err := helperkv.NewModifier(config.Insecure, readToken, config.VaultAddress, config.EnvRaw, config.Regions, true, config.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		config.Log.Println("Failure to init to vault")
		return nil, err
	}
	envParts := strings.Split(config.EnvRaw, "-")
	mod.Env = envParts[0]

	deploymentListData, deploymentListDataErr := mod.List("super-secrets/Index/TrcVault/trcplugin", config.Log)
	if deploymentListDataErr != nil {
		return nil, deploymentListDataErr
	}

	if deploymentListData == nil {
		return nil, errors.New("no plugins available")
	}
	deploymentList := []string{}

	for _, deploymentInterface := range deploymentListData.Data {
		for _, deploymentPath := range deploymentInterface.([]interface{}) {
			deployment := strings.TrimSuffix(deploymentPath.(string), "/")

			deploymentConfig, deploymentConfigErr := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			if deploymentConfigErr != nil || deploymentConfig == nil {
				continue
			}

			if deploymentConfig["trctype"].(string) == "trcshservice" {
				deploymentList = append(deploymentList, deployment)
			}
		}
	}

	return deploymentList, nil
}
