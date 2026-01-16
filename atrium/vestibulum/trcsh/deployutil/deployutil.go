package deployutil

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
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

// LoadPluginDeploymentScript - Loads a plugin's deploy.trc script directly from vault.
func LoadPluginDeploymentScript(trcshDriverConfig *capauth.TrcshDriverConfig, trcshConfig *capauth.TrcShConfig, pwd string) ([]byte, error) {
	if strings.Contains(pwd, "TrcDeploy") && trcshDriverConfig.DriverConfig.DeploymentConfig != nil && trcshDriverConfig.DriverConfig.DeploymentConfig != nil {
		if deployment, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcplugin"]; ok {
			if deploymentAlias, deployAliasOk := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcpluginalias"]; deployAliasOk {
				deployment = deploymentAlias
			}
			mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			// Swapping in project root...
			tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			approle := "bamboo"
			autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, &tokenName, nil, &trcshDriverConfig.DriverConfig.CoreConfig.Env, &mergedEnvBasis, &approle, false)
			if autoErr != nil {
				fmt.Fprintln(os.Stderr, "Missing auth components.")
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
				fmt.Fprintln(os.Stderr, "Unable to obtain resources for deployment")
				return nil, err
			}

			// DeploymentConfig is already initialized by TrcshInitConfig
			if _, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcdeploybasis"]; !ok {
				mod.Env = trcshDriverConfig.DriverConfig.CoreConfig.Env
			} else {
				mod.Env = trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
			}
			if trcDeployRoot, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcdeployroot"]; ok {
				trcshDriverConfig.DriverConfig.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
				trcshDriverConfig.DriverConfig.EndDir = trcDeployRoot.(string)
			}

			if trcProjectService, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcprojectservice"]; ok && strings.Contains(trcProjectService.(string), "/") {
				var content []byte
				trcProjectServiceSlice := strings.Split(trcProjectService.(string), "/")
				fmt.Fprintf(os.Stderr, "Loading deployment script for %s and env %s\n", deployment, mod.Env)
				deployScriptPath := fmt.Sprintf("./trc_templates/%s/deploy/deploy.trc.tmpl", trcProjectService.(string))
				subErr := MountPluginFileSystem(trcshDriverConfig.DriverConfig,
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
				fmt.Fprintln(os.Stderr, "Project not configured and ready for deployment.  Missing projectservice")
				return nil, errors.New("project not configured and ready for deployment.  missing projectservice")
			}
		}
	}
	return nil, errors.New("not deployer")
}

// Loads a plugin's deploy template from vault.
func MountPluginFileSystem(
	trcshDriverConfig *config.DriverConfig,
	trcPath string,
	projectService string,
) error {
	if !strings.Contains(trcPath, "/deploy/") {
		fmt.Fprintln(os.Stderr, "Trcsh - Failed to fetch template using projectServicePtr.  Path is missing /deploy/")
		return errors.New("trcsh - Failed to fetch template using projectServicePtr.  path is missing /deploy/")
	}
	mergedEnvBasis := trcshDriverConfig.CoreConfig.EnvBasis
	tokenNamePtr := trcshDriverConfig.CoreConfig.GetCurrentToken("config_token_%s")

	deployTrcPath := trcPath[strings.LastIndex(trcPath, "/deploy/"):]
	if trcIndex := strings.Index(deployTrcPath, ".trc"); trcIndex > 0 {
		deployTrcPath = deployTrcPath[0:trcIndex] // get rid of trailing .trc
	}
	templatePathsPtr := projectService + deployTrcPath
	trcshDriverConfig.EndDir = "./trc_templates"

	return trcsubbase.CommonMain(&trcshDriverConfig.CoreConfig.EnvBasis,
		&mergedEnvBasis, tokenNamePtr, nil, []string{"trcsh", "-templatePaths=" + templatePathsPtr}, trcshDriverConfig)
}

// Gets list of supported deployers for current environment.
func GetDeployers(kernelPluginHandler *hive.PluginHandler,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	deploymentShardsSet map[string]struct{},
	exeTypeFlags ...*bool,
) ([]*map[string]interface{}, error) {
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
	if !isShellRunner && !kernelopts.BuildOptions.IsKernel() {
		mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
		roleEntity := "bamboo"
		autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, tokenNamePtr, &tokenPtr, &trcshDriverConfig.DriverConfig.CoreConfig.Env, &mergedEnvBasis, &roleEntity, false)
		if autoErr != nil {
			fmt.Fprintln(os.Stderr, "Missing auth components.")
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
		fmt.Fprintln(os.Stderr, "Failure to init to vault")
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Failure to init to vault")
		return nil, err
	}
	envBasisParts := strings.Split(trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis, "-")
	mod.Env = envBasisParts[0]

	deploymentListData, deploymentListDataErr := mod.List("super-secrets/Index/TrcVault/trcplugin", trcshDriverConfig.DriverConfig.CoreConfig.Log)
	if deploymentListDataErr != nil {
		return nil, deploymentListDataErr
	}

	if deploymentListData == nil {
		return nil, errors.New("no plugins available")
	}
	deploymentList := []*map[string]interface{}{}
	var machineID string
	if isDrone && !isShellRunner && !kernelopts.BuildOptions.IsKernel() {
		machineID = coreopts.BuildOptions.GetMachineID()
		if len(machineID) == 0 {
			return nil, errors.New("unable to access id of machine")
		}
	}
	for _, deploymentInterface := range deploymentListData.Data {
		for _, deploymentPath := range deploymentInterface.([]any) {
			deployment := strings.TrimSuffix(deploymentPath.(string), "/")

			if len(deployment) == 0 {
				continue
			}
			if len(deploymentShardsSet) > 0 {
				if _, ok := deploymentShardsSet[deployment]; !ok {
					continue
				}
			}

			deploymentConfig, deploymentConfigErr := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
			if deploymentConfigErr != nil || deploymentConfig == nil {
				continue
			}
			if _, ok := deploymentConfig["trctype"]; !ok {
				continue
			}

			if _, ok := deploymentConfig["trcplugin"]; !ok {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Missing Certify entry, ignoring deployment: %s\n", deployment)
				continue
			}

			if trcplugin, ok := deploymentConfig["trcplugin"].(string); !ok || trcplugin != deployment {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Ignoring invalid deployment name: %s\n", deployment)
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
							splitID := strings.Split(id, ":")
							splitEnv := strings.Split(trcshDriverConfig.DriverConfig.CoreConfig.Env, "-")
							if len(splitID) == 1 && len(splitEnv) == 1 && len(splitID[0]) > 0 && len(splitEnv[0]) > 0 {
								valid_id = splitID[0]
								break
							} else if len(splitID) != 2 && len(splitEnv) != 2 && len(splitID[1]) > 0 && len(splitEnv[1]) > 0 {
								return nil, errors.New("unexpected type of deployer ids returned from vault for " + deployment)
							} else if len(splitEnv) > 1 && splitEnv[1] == splitID[0] {
								valid_id = splitID[1]
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
				isValidInstance := false
				if validEnv, ok := deploymentConfig["instances"]; ok && kernelopts.BuildOptions.IsKernel() {
					kernelID := kernelPluginHandler.GetKernelID()
					if instances, ok := validEnv.(string); ok {
						if len(instances) > 0 {
							instancesList := strings.Split(instances, ",")
							for _, instance := range instancesList {
								if instance == "*" {
									isValidInstance = true
									break
								} else {
									instanceKernelId := -1
									if len(instance) > 0 {
										instanceKernelId, _ = strconv.Atoi(instance)
									}

									if kernelID >= 0 && instanceKernelId >= 0 && instanceKernelId == kernelID {
										isValidInstance = true
										break
									}
								}
							}
						}
					} else {
						return nil, errors.New("unexpected type of instances returned from vault for " + deployment)
					}
				}
				if kernelopts.BuildOptions.IsKernel() && isValidInstance && (deploymentConfig["trctype"].(string) == "trcshpluginservice" || deploymentConfig["trctype"].(string) == "trcshkubeservice" || deploymentConfig["trctype"].(string) == "trcflowpluginservice") {
					deploymentList = append(deploymentList, &deploymentConfig)
				} else if (deploymentConfig["trctype"].(string) == "trcshservice" || deploymentConfig["trctype"].(string) == "trcflowpluginservice" || deploymentConfig["trctype"].(string) == "trcshpluginservice") && len(valid_id) > 0 && valid_id == machineID {
					deploymentList = append(deploymentList, &deploymentConfig)
				}
			} else {
				if trcshDriverConfig.DriverConfig.CoreConfig.IsEditor {
					deploymentList = append(deploymentList, &deploymentConfig)
				} else {
					if deploymentConfig["trctype"].(string) == "trcshcmdtoolplugin" || deploymentConfig["trctype"].(string) == "trcflowpluginservice" {
						deploymentList = append(deploymentList, &deploymentConfig)
					}
				}
			}
		}
	}
	return deploymentList, nil
}
