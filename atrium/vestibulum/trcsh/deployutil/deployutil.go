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
func LoadPluginDeploymentScript(driverConfig *eUtils.DriverConfig, trcshConfig *capauth.TrcShConfig, pwd string) ([]byte, error) {
	if strings.Contains(pwd, "TrcDeploy") && len(driverConfig.DeploymentConfig) > 0 {
		if deployment, ok := driverConfig.DeploymentConfig["trcplugin"]; ok {
			if deploymentAlias, deployAliasOk := driverConfig.DeploymentConfig["trcpluginalias"]; deployAliasOk {
				deployment = deploymentAlias
			}
			mergedEnvRaw := driverConfig.EnvRaw
			// Swapping in project root...
			configRoleSlice := strings.Split(*trcshConfig.ConfigRole, ":")
			tokenName := "config_token_" + driverConfig.EnvRaw
			readToken := ""
			autoErr := eUtils.AutoAuth(driverConfig, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &driverConfig.Env, &driverConfig.VaultAddress, &mergedEnvRaw, "driverConfig.yml", false)
			if autoErr != nil {
				fmt.Println("Missing auth components.")
				return nil, autoErr
			}

			mod, err := helperkv.NewModifier(driverConfig.Insecure, readToken, *trcshConfig.VaultAddress, driverConfig.EnvRaw, driverConfig.Regions, true, driverConfig.CoreConfig.Log)
			if err != nil {
				fmt.Println("Unable to obtain resources for deployment")
				return nil, err
			}
			tempEnv := driverConfig.EnvRaw
			envParts := strings.Split(driverConfig.EnvRaw, "-")
			mod.Env = envParts[0]
			fmt.Printf("Loading deployment details for %s and env %s", deployment, mod.Env)
			deploymentConfig, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			mod.Env = tempEnv
			if err != nil {
				fmt.Println("Unable to obtain config for deployment")
				return nil, err
			}
			deploymentConfig["trcpluginalias"] = deployment
			driverConfig.DeploymentConfig = deploymentConfig
			if trcDeployRoot, ok := driverConfig.DeploymentConfig["trcdeployroot"]; ok {
				driverConfig.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
				driverConfig.EndDir = trcDeployRoot.(string)
			}

			if trcProjectService, ok := driverConfig.DeploymentConfig["trcprojectservice"]; ok && strings.Contains(trcProjectService.(string), "/") {
				var content []byte
				trcProjectServiceSlice := strings.Split(trcProjectService.(string), "/")
				driverConfig.ZeroConfig = true
				contentArray, _, _, err := vcutils.ConfigTemplate(driverConfig, mod, fmt.Sprintf("./trc_templates/%s/deploy/deploy.trc.tmpl", trcProjectService.(string)), true, trcProjectServiceSlice[0], trcProjectServiceSlice[1], false, true)
				driverConfig.ZeroConfig = false
				if err != nil {
					eUtils.LogErrorObject(&driverConfig.CoreConfig, err, false)
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
func GetDeployers(driverConfig *eUtils.DriverConfig) ([]string, error) {

	// Swapping in project root...
	configRoleSlice := strings.Split(driverConfig.AppRoleConfig, ":")
	mergedEnvRaw := driverConfig.EnvRaw
	tokenName := "config_token_" + driverConfig.Env
	readToken := ""
	autoErr := eUtils.AutoAuth(driverConfig, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &driverConfig.Env, &driverConfig.VaultAddress, &mergedEnvRaw, "config.yml", false)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return nil, autoErr
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, &readToken)
	}

	mod, err := helperkv.NewModifier(driverConfig.Insecure, readToken, driverConfig.VaultAddress, driverConfig.EnvRaw, driverConfig.Regions, true, driverConfig.CoreConfig.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		driverConfig.CoreConfig.Log.Println("Failure to init to vault")
		return nil, err
	}
	envParts := strings.Split(driverConfig.EnvRaw, "-")
	mod.Env = envParts[0]

	deploymentListData, deploymentListDataErr := mod.List("super-secrets/Index/TrcVault/trcplugin", driverConfig.CoreConfig.Log)
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
