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
func LoadPluginDeploymentScript(trcshDriverConfig *capauth.TrcshDriverConfig, trcshConfig *capauth.TrcShConfig, pwd string) ([]byte, error) {
	if strings.Contains(pwd, "TrcDeploy") && len(trcshDriverConfig.DriverConfig.DeploymentConfig) > 0 {
		if deployment, ok := trcshDriverConfig.DriverConfig.DeploymentConfig["trcplugin"]; ok {
			if deploymentAlias, deployAliasOk := trcshDriverConfig.DriverConfig.DeploymentConfig["trcpluginalias"]; deployAliasOk {
				deployment = deploymentAlias
			}
			mergedEnvBasis := trcshDriverConfig.DriverConfig.EnvBasis
			// Swapping in project root...
			configRoleSlice := strings.Split(*trcshConfig.ConfigRole, ":")
			tokenName := "config_token_" + trcshDriverConfig.DriverConfig.EnvBasis
			readToken := ""
			autoErr := eUtils.AutoAuth(&trcshDriverConfig.DriverConfig, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &trcshDriverConfig.DriverConfig.Env, &trcshDriverConfig.DriverConfig.VaultAddress, &mergedEnvBasis, "config.yml", false)
			if autoErr != nil {
				fmt.Println("Missing auth components.")
				return nil, autoErr
			}

			mod, err := helperkv.NewModifier(trcshDriverConfig.DriverConfig.Insecure, readToken, *trcshConfig.VaultAddress, trcshDriverConfig.DriverConfig.EnvBasis, trcshDriverConfig.DriverConfig.Regions, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
			if err != nil {
				fmt.Println("Unable to obtain resources for deployment")
				return nil, err
			}
			tempEnv := trcshDriverConfig.DriverConfig.EnvBasis
			envParts := strings.Split(trcshDriverConfig.DriverConfig.EnvBasis, "-")
			mod.Env = envParts[0]
			fmt.Printf("Loading deployment details for %s and env %s", deployment, mod.Env)
			deploymentConfig, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			mod.Env = tempEnv
			if err != nil {
				fmt.Println("Unable to obtain config for deployment")
				return nil, err
			}
			deploymentConfig["trcpluginalias"] = deployment
			trcshDriverConfig.DriverConfig.DeploymentConfig = deploymentConfig
			if trcDeployRoot, ok := trcshDriverConfig.DriverConfig.DeploymentConfig["trcdeployroot"]; ok {
				trcshDriverConfig.DriverConfig.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
				trcshDriverConfig.DriverConfig.EndDir = trcDeployRoot.(string)
			}

			if trcProjectService, ok := trcshDriverConfig.DriverConfig.DeploymentConfig["trcprojectservice"]; ok && strings.Contains(trcProjectService.(string), "/") {
				var content []byte
				trcProjectServiceSlice := strings.Split(trcProjectService.(string), "/")
				trcshDriverConfig.DriverConfig.ZeroConfig = true
				contentArray, _, _, err := vcutils.ConfigTemplate(&trcshDriverConfig.DriverConfig, mod, fmt.Sprintf("./trc_templates/%s/deploy/deploy.trc.tmpl", trcProjectService.(string)), true, trcProjectServiceSlice[0], trcProjectServiceSlice[1], false, true)
				trcshDriverConfig.DriverConfig.ZeroConfig = false
				if err != nil {
					eUtils.LogErrorObject(&trcshDriverConfig.DriverConfig.CoreConfig, err, false)
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
func GetDeployers(trcshDriverConfig *capauth.TrcshDriverConfig) ([]string, error) {

	// Swapping in project root...
	configRoleSlice := strings.Split(trcshDriverConfig.DriverConfig.AppRoleConfig, ":")
	mergedEnvBasis := trcshDriverConfig.DriverConfig.EnvBasis
	tokenName := "config_token_" + trcshDriverConfig.DriverConfig.Env
	readToken := ""
	autoErr := eUtils.AutoAuth(&trcshDriverConfig.DriverConfig, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &trcshDriverConfig.DriverConfig.Env, &trcshDriverConfig.DriverConfig.VaultAddress, &mergedEnvBasis, "config.yml", false)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return nil, autoErr
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, &readToken)
	}

	mod, err := helperkv.NewModifier(trcshDriverConfig.DriverConfig.Insecure, readToken, trcshDriverConfig.DriverConfig.VaultAddress, trcshDriverConfig.DriverConfig.EnvBasis, trcshDriverConfig.DriverConfig.Regions, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Failure to init to vault")
		return nil, err
	}
	envParts := strings.Split(trcshDriverConfig.DriverConfig.EnvBasis, "-")
	mod.Env = envParts[0]

	deploymentListData, deploymentListDataErr := mod.List("super-secrets/Index/TrcVault/trcplugin", trcshDriverConfig.DriverConfig.CoreConfig.Log)
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
