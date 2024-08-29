package deployutil

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
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
			mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			// Swapping in project root...
			configRoleSlice := strings.Split(*trcshConfig.ConfigRole, ":")
			tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			readToken := ""
			autoErr := eUtils.AutoAuth(&trcshDriverConfig.DriverConfig, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &trcshDriverConfig.DriverConfig.CoreConfig.Env, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &mergedEnvBasis, "config.yml", false)
			if autoErr != nil {
				fmt.Println("Missing auth components.")
				return nil, autoErr
			}

			mod, err := helperkv.NewModifier(trcshDriverConfig.DriverConfig.CoreConfig.Insecure, readToken, *trcshConfig.VaultAddress, trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis, trcshDriverConfig.DriverConfig.CoreConfig.Regions, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
			if err != nil {
				fmt.Println("Unable to obtain resources for deployment")
				return nil, err
			}
			mod.Env = trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			fmt.Printf("Loading deployer details for %s and env %s\n", deployment, mod.EnvBasis)
			deploymentConfig, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			if _, ok := deploymentConfig["trcdeploybasis"]; !ok {
				// Whether to load agent deployment script from env basis instead of provided env.
				// By default, we always use agent provided env to load the script.
				// In presence of trcdeploybasis, we'll leave the mod Env as EnvBasis and continue.
				mod.Env = trcshDriverConfig.DriverConfig.CoreConfig.Env
			}
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
				fmt.Printf("Loading deployment script for %s and env %s\n", deployment, mod.Env)
				contentArray, _, _, err := vcutils.ConfigTemplate(&trcshDriverConfig.DriverConfig, mod, fmt.Sprintf("./trc_templates/%s/deploy/deploy.trc.tmpl", trcProjectService.(string)), true, trcProjectServiceSlice[0], trcProjectServiceSlice[1], false, true)
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
func GetDeployers(trcshDriverConfig *capauth.TrcshDriverConfig, dronePtr ...*bool) ([]string, error) {
	isDrone := false
	if len(dronePtr) > 0 {
		isDrone = *dronePtr[0]
	}
	// Swapping in project root...
	configRoleSlice := strings.Split(trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfig, ":")
	mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.Env
	readToken := ""
	autoErr := eUtils.AutoAuth(&trcshDriverConfig.DriverConfig, &configRoleSlice[1], &configRoleSlice[0], &readToken, &tokenName, &trcshDriverConfig.DriverConfig.CoreConfig.Env, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &mergedEnvBasis, "config.yml", false)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		return nil, autoErr
	}
	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		memprotectopts.MemProtect(nil, &readToken)
	}

	mod, err := helperkv.NewModifier(trcshDriverConfig.DriverConfig.CoreConfig.Insecure, readToken, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis, trcshDriverConfig.DriverConfig.CoreConfig.Regions, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
	if mod != nil {
		defer mod.Release()
	}
	if err != nil {
		fmt.Println("Failure to init to vault")
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Failure to init to vault")
		return nil, err
	}
	envParts := strings.Split(trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis, "-")
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
			if isDrone {
				ip_address, err := getLocalIP()
				if err != nil {
					return nil, errors.New("unable to access ip address of machine")
				}
				ip_addr := ip_address.String()
				var valid_ip string
				if addresses, ok := deploymentConfig["trcdeployeraddr"]; ok {
					if addrs, ok := addresses.(string); ok {
						splitAddrs := strings.Split(addrs, ",")
						for _, addr := range splitAddrs {
							splitAddr := strings.Split(addr, ":")
							splitEnv := strings.Split(trcshDriverConfig.DriverConfig.CoreConfig.Env, "-")
							if len(splitAddr) == 1 && len(splitEnv) == 1 && len(splitAddr[0]) > 0 && len(splitEnv[0]) > 0 {
								valid_ip = splitAddr[0]
								break
							} else if len(splitAddr) != 2 && len(splitEnv) != 2 && len(splitAddr[1]) > 0 && len(splitEnv[1]) > 0 {
								return nil, errors.New("unexpected type of deployer addresses returned from vault for " + deployment)
							} else if splitEnv[1] == splitAddr[0] {
								valid_ip = splitAddr[1]
								break
							}
						}
						if len(valid_ip) == 0 {
							return nil, errors.New("no deployer address specified for environment from vault for " + deployment)
						}
					} else {
						return nil, errors.New("unexpected type of deployer addresses returned from vault for " + deployment)
					}
				}

				// if  {
				if coreopts.BuildOptions.IsKernel() && deploymentConfig["trctype"].(string) == "trcshpluginservice" {
					deploymentList = append(deploymentList, deployment)
				} else if deploymentConfig["trctype"].(string) == "trcshservice" && len(valid_ip) > 0 && valid_ip == ip_addr {
					deploymentList = append(deploymentList, deployment)
				}
				// }
			}
		}
	}
	return deploymentList, nil
}

func getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddress := conn.LocalAddr().(*net.UDPAddr)

	return localAddress.IP, nil
}
