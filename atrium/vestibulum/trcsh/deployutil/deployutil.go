package deployutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"gopkg.in/fsnotify.v1"
)

const (
	KERNEL_PIDFILE = "/tmp/trcshk.pid"
)

// Watches for pidfile deletion and exits  Used by kubernetes to manage pods
func KernelShutdownWatcher(logger *log.Logger) {
	if _, err := os.Stat(KERNEL_PIDFILE); os.IsNotExist(err) {
		_, mkErr := os.Create(KERNEL_PIDFILE)
		if mkErr != nil {
			logger.Println("Unable to create pidfile.")
			return
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Printf("Unable to set up watcher: %s\n", err.Error())
		return
	}
	defer watcher.Close()

	// Setting up forever loop
	go func(l *log.Logger) {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					os.Exit(0)
				}
			case err := <-watcher.Errors:
				l.Printf("Pidfile watch error: %s.  Shutting down\n", err.Error())
				os.Exit(0)
			}
		}
	}(logger)

	err = watcher.Add(KERNEL_PIDFILE)
	if err != nil {
		logger.Printf("Can't watch pidfile: %s", err.Error())
		return
	}
	keepAliveChan := make(chan bool)
	keepAliveChan <- true
}

// Loads a plugin's deploy.trc script directly from vault.
func LoadPluginDeploymentScript(trcshDriverConfig *capauth.TrcshDriverConfig, trcshConfig *capauth.TrcShConfig, pwd string) ([]byte, error) {
	if strings.Contains(pwd, "TrcDeploy") && len(trcshDriverConfig.DriverConfig.DeploymentConfig) > 0 {
		if deployment, ok := trcshDriverConfig.DriverConfig.DeploymentConfig["trcplugin"]; ok {
			if deploymentAlias, deployAliasOk := trcshDriverConfig.DriverConfig.DeploymentConfig["trcpluginalias"]; deployAliasOk {
				deployment = deploymentAlias
			}
			mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			// Swapping in project root...
			tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			approle := "config.yml"
			tokenPtr := new(string)
			autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, &tokenName, &tokenPtr, &trcshDriverConfig.DriverConfig.CoreConfig.Env, &mergedEnvBasis, &approle, false)
			if autoErr != nil {
				fmt.Println("Missing auth components.")
				return nil, autoErr
			}

			mod, err := helperkv.NewModifier(
				trcshDriverConfig.DriverConfig.CoreConfig.Insecure,
				trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken(tokenName),
				trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.VaultAddressPtr,
				trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis,
				trcshDriverConfig.DriverConfig.CoreConfig.Regions,
				true,
				trcshDriverConfig.DriverConfig.CoreConfig.Log)
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
				deployScriptPath := fmt.Sprintf("./trc_templates/%s/deploy/deploy.trc.tmpl", trcProjectService.(string))
				subErr := MountPluginFileSystem(trcshDriverConfig,
					deployScriptPath,
					trcProjectService.(string))
				if subErr != nil {
					eUtils.LogErrorObject(trcshDriverConfig.DriverConfig.CoreConfig, subErr, false)
					return nil, subErr
				}
				contentArray, _, _, err := vcutils.ConfigTemplate(trcshDriverConfig.DriverConfig, mod, deployScriptPath, true, trcProjectServiceSlice[0], trcProjectServiceSlice[1], false, true)
				if err != nil {
					eUtils.LogErrorObject(trcshDriverConfig.DriverConfig.CoreConfig, err, false)
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

// Loads a plugin's deploy template from vault.
func MountPluginFileSystem(
	trcshDriverConfig *capauth.TrcshDriverConfig,
	trcPath string,
	projectService string) error {

	if !strings.Contains(trcPath, "/deploy/") {
		fmt.Println("Trcsh - Failed to fetch template using projectServicePtr.  Path is missing /deploy/")
		return errors.New("trcsh - Failed to fetch template using projectServicePtr.  path is missing /deploy/")
	}
	mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	tokenNamePtr := trcshDriverConfig.DriverConfig.CoreConfig.GetCurrentToken("config_token_%s")

	deployTrcPath := trcPath[strings.LastIndex(trcPath, "/deploy/"):]
	if trcIndex := strings.Index(deployTrcPath, ".trc"); trcIndex > 0 {
		deployTrcPath = deployTrcPath[0:trcIndex] // get rid of trailing .trc
	}
	templatePathsPtr := projectService + deployTrcPath
	trcshDriverConfig.DriverConfig.EndDir = "./trc_templates"

	return trcsubbase.CommonMain(&trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis,
		&mergedEnvBasis, tokenNamePtr, nil, []string{"trcsh", "-templatePaths=" + templatePathsPtr}, trcshDriverConfig.DriverConfig)
}

// Gets list of supported deployers for current environment.
func GetDeployers(trcshDriverConfig *capauth.TrcshDriverConfig, exeTypeFlags ...*bool) ([]string, error) {
	isDrone := false
	isShellRunner := false
	if len(exeTypeFlags) > 0 {
		isDrone = *exeTypeFlags[0]
		if len(exeTypeFlags) > 1 {
			isShellRunner = *exeTypeFlags[1]
		}
	}
	// Swapping in project root...
	tokenPtr := new(string)
	tokenNamePtr := trcshDriverConfig.DriverConfig.CoreConfig.GetCurrentToken("config_token_%s")
	if !isShellRunner {
		mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
		roleEntity := "config.yml"
		autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, tokenNamePtr, &tokenPtr, &trcshDriverConfig.DriverConfig.CoreConfig.Env, &mergedEnvBasis, &roleEntity, false)
		if autoErr != nil {
			fmt.Println("Missing auth components.")
			return nil, autoErr
		}
	} else {
		tokenNamePtr = trcshDriverConfig.DriverConfig.CoreConfig.CurrentTokenNamePtr
	}

	mod, err := helperkv.NewModifierFromCoreConfig(trcshDriverConfig.DriverConfig.CoreConfig,
		*tokenNamePtr,
		trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis,
		true)
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
	var machineID string
	if isDrone && !isShellRunner && !kernelopts.BuildOptions.IsKernel() {
		machineID = coreopts.BuildOptions.GetMachineID()
		if len(machineID) == 0 {
			return nil, errors.New("unable to access id of machine")
		}
	}
	for _, deploymentInterface := range deploymentListData.Data {
		for _, deploymentPath := range deploymentInterface.([]interface{}) {
			deployment := strings.TrimSuffix(deploymentPath.(string), "/")

			if len(deployment) == 0 {
				continue
			}

			deploymentConfig, deploymentConfigErr := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			if deploymentConfigErr != nil || deploymentConfig == nil {
				continue
			}
			if _, ok := deploymentConfig["trctype"]; !ok {
				continue
			}

			if isDrone && !isShellRunner {
				var valid_id string
				if deployerids, ok := deploymentConfig["trcdeployerids"]; ok {
					if ids, ok := deployerids.(string); ok {
						if len(ids) == 0 {
							trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Deployment %s lacks deployer ids\n", deployment)
							continue
						}
						splitIds := strings.Split(ids, ",")
						for _, id := range splitIds {
							splitId := strings.Split(id, ":")
							splitEnv := strings.Split(trcshDriverConfig.DriverConfig.CoreConfig.Env, "-")
							if len(splitId) == 1 && len(splitEnv) == 1 && len(splitId[0]) > 0 && len(splitEnv[0]) > 0 {
								valid_id = splitId[0]
								break
							} else if len(splitId) != 2 && len(splitEnv) != 2 && len(splitId[1]) > 0 && len(splitEnv[1]) > 0 {
								return nil, errors.New("unexpected type of deployer ids returned from vault for " + deployment)
							} else if len(splitEnv) > 1 && splitEnv[1] == splitId[0] {
								valid_id = splitId[1]
								break
							}
						}
						if len(valid_id) == 0 {
							trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Deployment %s lacks deployer ids\n", deployment)
							continue
						}
					} else {
						return nil, errors.New("unexpected type of deployer ids returned from vault for " + deployment)
					}
				}
				if kernelopts.BuildOptions.IsKernel() && deploymentConfig["trctype"].(string) == "trcshpluginservice" || deploymentConfig["trctype"].(string) == "trcshkubeservice" || deploymentConfig["trctype"].(string) == "trcflowpluginservice" {
					deploymentList = append(deploymentList, deployment)
				} else if (deploymentConfig["trctype"].(string) == "trcshservice" || deploymentConfig["trctype"].(string) == "trcflowpluginservice") && len(valid_id) > 0 && valid_id == machineID {
					deploymentList = append(deploymentList, deployment)
				}
			} else {
				if deploymentConfig["trctype"].(string) == "trcshcmdtoolplugin" || deploymentConfig["trctype"].(string) == "trcflowpluginservice" {
					deploymentList = append(deploymentList, deployment)
				}
			}
		}
	}
	return deploymentList, nil
}
