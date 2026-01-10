package trcshbase

import (
	"bytes"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/danieljoos/wincred"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memonly"
	"github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	trcshmemfs "github.com/trimble-oss/tierceron-core/v2/trcshfs"
	"github.com/trimble-oss/tierceron-core/v2/trcshfs/trcshio"
	"github.com/trimble-oss/tierceron-hat/cap"
	captiplib "github.com/trimble-oss/tierceron-hat/captip/captiplib"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/deployutil"
	kube "github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/trcshauth"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"gopkg.in/yaml.v2"
)

var (
	gAgentConfig        *capauth.AgentConfigs = nil
	gTrcshConfig        *capauth.TrcShConfig
	kernelPluginHandler *hive.PluginHandler = nil
)

var MODE_PERCH_STR string = string([]byte{cap.MODE_PERCH})

const (
	YOU_SHALL_NOT_PASS = "you shall not pass"
)

func CreateLogFile() (*log.Logger, error) {
	var f *os.File
	var logPrefix string = "[DEPLOY]"
	if kernelopts.BuildOptions.IsKernel() {
		logPrefix = "[trcshk]"
		f = os.Stdout
	} else {
		logFile := "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
		if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"deploy.log" {
			logFile = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
		}
		var errOpenFile error
		f, errOpenFile = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if errOpenFile != nil {
			return nil, errOpenFile
		}
	}
	logger := log.New(f, logPrefix, log.LstdFlags)
	return logger, nil
}

func TrcshInitConfig(driverConfigPtr *config.DriverConfig,
	env string, region string,
	pathParam string,
	useMemCache bool,
	outputMemCache bool,
	isShell bool,
	deploymentConfig *map[string]any,
	isTraceless bool,
	logger ...*log.Logger,
) (*capauth.TrcshDriverConfig, error) {
	if len(env) == 0 {
		env = os.Getenv("TRC_ENV")
	}
	if len(region) == 0 {
		region = os.Getenv("TRC_REGION")
	}

	regions := []string{}
	if (kernelopts.BuildOptions != nil && kernelopts.BuildOptions.IsKernel()) || strings.HasPrefix(env, "staging") || strings.HasPrefix(env, "prod") || strings.HasPrefix(env, "dev") {
		if isTraceless && strings.Contains(env, "staging") {
			// Dev environments have a 'staging' launch area.
			// traceless mode allows this special staging environment to work in dev environment.
			prod.SetProd(false)
		} else {
			prod.SetProdByEnv(env)
		}

		supportedRegions := eUtils.GetSupportedProdRegions()
		if region != "" {
			for _, supportedRegion := range supportedRegions {
				if region == supportedRegion {
					regions = append(regions, region)
					break
				}
			}
			if len(regions) == 0 {
				fmt.Fprintln(os.Stderr, "Unsupported region: "+region)
				return nil, errors.New("Unsupported region: " + region)
			}
		}
	}

	// Check if logger passed in - if not call create log method that does following below...
	var providedLogger *log.Logger
	var err error
	if len(logger) == 0 && (driverConfigPtr == nil || driverConfigPtr.CoreConfig.Log == nil) {
		providedLogger, err = CreateLogFile()
		if err != nil {
			return nil, err
		}
	} else {
		if driverConfigPtr != nil && driverConfigPtr.CoreConfig.Log != nil {
			providedLogger = driverConfigPtr.CoreConfig.Log
		} else {
			providedLogger = logger[0]
		}
	}
	var trcshMemFs trcshio.MemoryFileSystem = nil
	var shellRunner func(*config.DriverConfig, string, string)
	isEditor := false
	isDrone := false

	if driverConfigPtr != nil {
		if driverConfigPtr.CoreConfig != nil {
			if driverConfigPtr.CoreConfig.TokenCache != nil &&
				!(*driverConfigPtr).CoreConfig.TokenCache.IsEmpty() &&
				gTokenCache.IsEmpty() {
				gTokenCache = (*driverConfigPtr).CoreConfig.TokenCache
			}
			if driverConfigPtr.CoreConfig.Regions == nil && len(regions) > 0 {
				driverConfigPtr.CoreConfig.Regions = regions
			}
			isEditor = driverConfigPtr.CoreConfig.IsEditor

		}
		shellRunner = driverConfigPtr.ShellRunner
		trcshMemFs = driverConfigPtr.MemFs
		isDrone = driverConfigPtr.IsDrone
	}
	if !isEditor {
		fmt.Fprintln(os.Stderr, "trcsh env: "+env)
		fmt.Fprintf(os.Stderr, "trcsh regions: %s\n", strings.Join(regions, ", "))
	}

	if trcshMemFs == nil {
		trcshMemFs = trcshmemfs.NewTrcshMemFs()
	}

	trcshDriverConfig := &capauth.TrcshDriverConfig{
		DriverConfig: &config.DriverConfig{
			CoreConfig: &coreconfig.CoreConfig{
				IsShell:       isShell,
				IsEditor:      isEditor,
				TokenCache:    gTokenCache,
				Insecure:      false,
				Env:           env,
				EnvBasis:      eUtils.GetEnvBasis(env),
				Regions:       regions,
				ExitOnFailure: true,
				Log:           providedLogger,
			},
			IsDrone:           isDrone,
			IsShellSubProcess: false,
			ShellRunner:       shellRunner,
			ReadMemCache:      useMemCache,
			SubOutputMemCache: useMemCache,
			OutputMemCache:    outputMemCache,
			MemFs:             trcshMemFs,
			ZeroConfig:        true,
			PathParam:         pathParam, // Make available to trcplgtool
		},
	}
	if driverConfigPtr != nil {
		if driverConfigPtr.CoreConfig != nil {
			trcshDriverConfig.DriverConfig.CoreConfig.CurrentTokenNamePtr = driverConfigPtr.CoreConfig.CurrentTokenNamePtr
		}
	}
	if deploymentConfig != nil {
		trcshDriverConfig.DriverConfig.DeploymentConfig = deploymentConfig
	}

	return trcshDriverConfig, nil
}

// Logging of deployer controller activities..
func deployerCtlEmote(featherCtx *cap.FeatherContext, ctlFlapMode string, msg string) {
	if strings.HasSuffix(ctlFlapMode, cap.CTL_COMPLETE) {
		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
		if eUtils.IsWindows() {
			featherCtx.Log.Println("Deployment controller complete")
			return
		} else {
			eUtils.LogSyncAndExit(featherCtx.Log, "Deployment controller complete - exiting with code 0\n", 0)
		}
	}

	if len(ctlFlapMode) > 0 && ctlFlapMode[0] == cap.MODE_FLAP {
		fmt.Fprintf(os.Stderr, "%s\n", msg)
	}
	deployerID, _ := deployopts.BuildOptions.GetDecodedDeployerId(*featherCtx.SessionIdentifier)
	featherCtx.Log.Printf("deployer: %s ctl: %s  msg: %s\n", deployerID, ctlFlapMode, strings.Trim(msg, "\n"))
	if strings.Contains(msg, "encountered errors") {
		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
		if eUtils.IsWindows() {
			featherCtx.Log.Println("Deployment encountered errors")
			return
		} else {
			eUtils.LogSyncAndExit(featherCtx.Log, "Deployment encountered errors - exiting with code 0\n", 0)
		}
	}
}

// Logging of deployer activities..
func deployerEmote(featherCtx *cap.FeatherContext, ctlFlapMode []byte, msg string) {
	if len(ctlFlapMode) > 0 && ctlFlapMode[0] != cap.MODE_PERCH && msg != captiplib.MSG_PERCH_AND_GAZE {
		featherCtx.Log.Printf("%s\n", msg)
	}
}

func deployCtlAcceptRemote(featherCtx *cap.FeatherContext, x int, y string) (bool, error) {
	return acceptInterruptFun(featherCtx, featherCtx.MultiSecondInterruptTicker, featherCtx.FifteenSecondInterruptTicker, featherCtx.ThirtySecondInterruptTicker)
}

func deployCtlAcceptRemoteNoTimeout(featherCtx *cap.FeatherContext, x int, y string) (bool, error) {
	return acceptInterruptNoTimeoutFun(featherCtx, featherCtx.MultiSecondInterruptTicker)
}

// deployCtl -- is the deployment controller or manager if you will.
func deployCtlInterrupted(featherCtx *cap.FeatherContext) error {
	eUtils.LogSyncAndExit(featherCtx.Log, "Deployment controller interrupted - exiting with code -1", -1)
	return nil
}

func deployerAcceptRemoteNoTimeout(featherCtx *cap.FeatherContext, x int, y string) (bool, error) {
	return acceptInterruptNoTimeoutFun(featherCtx, featherCtx.MultiSecondInterruptTicker)
}

// deployer -- does the work of deploying..
func deployerInterrupted(featherCtx *cap.FeatherContext) error {
	cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
	return nil
}

var gTokenCache *cache.TokenCache

// EnableDeployer initializes and starts running deployer for the provided deployment and environment.
func EnableDeployer(driverConfigPtr *config.DriverConfig,
	env string, region string,
	token string,
	trcPath string,
	useMemCache bool,
	outputMemCache bool,
	dronePtr *bool,
	deploymentConfig *map[string]any,
	tracelessPtr *bool,
	projectService ...*string,
) {
	trcshDriverConfig, err := TrcshInitConfig(driverConfigPtr,
		env,
		region,
		"",
		useMemCache,
		outputMemCache,
		false, // isShell
		deploymentConfig,
		*tracelessPtr,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initialization setup error: %s\n", err.Error())
	}
	if trcshDriverConfig.DriverConfig.DeploymentConfig != nil {
		// DeploymentConfig was copied from driverConfigPtr
		trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan = make(chan string, 20)
		if trcPlugin, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcplugin"]; ok {
			if deployment, isString := trcPlugin.(string); isString {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Starting deployer: %s\n", deployment)
			}
		}
	}

	//
	// Each deployer needs it's own context.
	//
	localHostAddr := ""
	var sessionIDentifier string
	var deployment string
	if trcPlugin, ok := (*deploymentConfig)["trcplugin"]; ok {
		if deploymentName, isString := trcPlugin.(string); isString {
			deployment = deploymentName
		}
	}
	if sessionID, ok := deployopts.BuildOptions.GetEncodedDeployerId(deployment, *gAgentConfig.Env); ok {
		sessionIDentifier = sessionID
	} else {
		eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("Unsupported deployer: %s\n", deployment), 0)
	}
	if !gTrcshConfig.IsShellRunner {
		trcshDriverConfig.FeatherCtx = captiplib.FeatherCtlInit(interruptChan,
			&localHostAddr,
			gAgentConfig.EncryptPass,
			gAgentConfig.EncryptSalt,
			gAgentConfig.HostAddr,
			gAgentConfig.HandshakeCode,
			&sessionIDentifier, /*Session identifier */
			&env,
			deployerAcceptRemoteNoTimeout,
			deployerInterrupted)
		trcshDriverConfig.FeatherCtx.Log = trcshDriverConfig.DriverConfig.CoreConfig.Log
		// featherCtx initialization is delayed for the self contained deployments (kubernetes, etc...)
		atomic.StoreInt64(&trcshDriverConfig.FeatherCtx.RunState, cap.RUN_STARTED)

		go captiplib.FeatherCtlEmitter(trcshDriverConfig.FeatherCtx, trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan, deployerEmote, nil)
	}
	projServ := ""
	if len(projectService) > 0 && projectService[0] != nil && kernelopts.BuildOptions.IsKernel() {
		projServ = *projectService[0]
	}

	go ProcessDeploy(trcshDriverConfig.FeatherCtx, trcshDriverConfig, trcPath, projServ, dronePtr)
}

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.

func CommonMain(envPtr *string, envCtxPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	configMap *map[string]any,
	driverConfigPtr *config.DriverConfig,
) error {
	if flagset == nil {
		flagset = flag.NewFlagSet(argLines[0], flag.ExitOnError)
		flagset.Usage = func() {
			fmt.Fprintf(flagset.Output(), "Usage of %s:\n", argLines[0])
			flagset.PrintDefaults()
		}
		flagset.String("env", "dev", "Environment to configure")
		flagset.String("addr", "", "API endpoint for the vault")
		flagset.String("secretID", "", "Secret for app role ID")
		flagset.String("appRoleID", "", "Public app role ID")
	} else {
		if driverConfigPtr != nil && driverConfigPtr.CoreConfig != nil && driverConfigPtr.CoreConfig.IsEditor {
			flagset.String("env", "dev", "Environment to configure")
		}
	}

	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	eUtils.InitHeadless(true)
	var regionPtr, trcPathPtr, projectServicePtr *string
	var dronePtr *bool
	var projectServiceFlagPtr *string
	var pluginNamePtr *string
	var droneFlagPtr *bool
	// Initiate signal handling.
	var ic chan os.Signal = make(chan os.Signal, 5)

	regionPtr = flagset.String("region", "", "Region to be processed")  // If this is blank -> use context otherwise override context.
	trcPathPtr = flagset.String("c", "", "Optional script to execute.") // If this is blank -> use context otherwise override context.
	projectServiceFlagPtr = flagset.String("projectService", "", "Service namespace to pull templates from if not present in LFS")
	pluginNamePtr = flagset.String("pluginName", "", "Plugin name (optional for some operations)")
	tracelessPtr := flagset.Bool("traceless", false, "Trace less") // For running with staging env and dev behavior
	droneFlagPtr = flagset.Bool("drone", false, "Run as drone.")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	isShellRunner := (configMap != nil)
	isShell := false

	// Initialize the token cache
	gTokenCache = driverConfigPtr.CoreConfig.TokenCache

	if !eUtils.IsWindows() {
		if os.Geteuid() == 0 {
			fmt.Fprintln(os.Stderr, "Trcsh cannot be run as root.")
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "ERROR: Trcsh cannot be run as root", -1)
		} else {
			if (len(os.Args) > 1 && strings.HasSuffix(os.Args[1], ".trc")) && (driverConfigPtr == nil || driverConfigPtr.CoreConfig == nil || !driverConfigPtr.CoreConfig.IsEditor) {
				isShell = true
				util.CheckNotSudo()
			}
		}

		if len(os.Args) > 1 {
			if strings.HasSuffix(os.Args[1], ".trc") && !strings.Contains(os.Args[1], "-c") {
				// Running as shell.
				os.Args[1] = "-c=" + os.Args[1]
			}
		}
		signal.Notify(ic, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT)
	} else {
		dronePtr = new(bool)
		*dronePtr = true
		driverConfigPtr.IsDrone = *dronePtr

		signal.Notify(ic, os.Interrupt)
	}
	go func() {
		x := <-ic
		interruptChan <- x
	}()

	flagset.Parse(argLines[1:])
	driverConfigPtr.CoreConfig.TokenCache.SetVaultAddress(addrPtr)

	if kernelopts.BuildOptions.IsKernel() && !driverConfigPtr.CoreConfig.IsEditor {
		dronePtr = new(bool)
		*dronePtr = true
	} else {
		if dronePtr == nil || !*dronePtr {
			dronePtr = droneFlagPtr
		}
	}
	projectServicePtr = projectServiceFlagPtr

	if isShell && !*dronePtr && (projectServicePtr == nil || len(*projectServicePtr) == 0) {
		eUtils.LogSyncAndExit(nil, "Script exiting, projectService flag is required", -1)
	}

	if isShell || (!*dronePtr && !isShellRunner) {
		if driverConfigPtr.CoreConfig.TokenCache.GetRole("hivekernel") == nil {
			deployRole := os.Getenv("DEPLOY_ROLE")
			deploySecret := os.Getenv("DEPLOY_SECRET")
			if len(deployRole) > 0 && len(deploySecret) > 0 {
				azureDeployRole := []string{deployRole, deploySecret}
				driverConfigPtr.CoreConfig.TokenCache.AddRole("hivekernel", &azureDeployRole)
				driverConfigPtr.CoreConfig.TokenCache.AddRole("deployauth", &azureDeployRole)
			}
		}

		pathParam := os.Getenv("PATH_PARAM")
		trcshDriverConfig, err := TrcshInitConfig(driverConfigPtr,
			*envPtr,
			*regionPtr,
			pathParam,
			true, // useMemCache
			true, // outputMemCache
			true, // isShell
			nil,  // DeploymentConfig
			*tracelessPtr,
		)
		if err != nil {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("ERROR: trcsh config setup failure: %s", err.Error()), 124)
		}
		if *tracelessPtr && *envPtr == "staging" {
			// Dev environments have a 'staging' launch area.
			// traceless mode allows this special staging environment to work in dev environment.
			prod.SetProd(false)
		}
		trcshDriverConfig.PluginName = *pluginNamePtr

		// Open deploy script and parse it.
		ProcessDeploy(nil, trcshDriverConfig, *trcPathPtr, *projectServicePtr, dronePtr)
	} else {
		if driverConfigPtr != nil && driverConfigPtr.CoreConfig.Log == nil {
			logger, err := CreateLogFile()
			if err != nil {
				eUtils.LogSyncAndExit(nil, fmt.Sprintf("Error initializing log file: %s\n", err.Error()), -1)
			}
			driverConfigPtr.CoreConfig.Log = logger
		}

		if kernelopts.BuildOptions.IsKernel() {
			go deployutil.KernelShutdownWatcher(driverConfigPtr.CoreConfig.Log)
		}
		var agentEnv string
		var deploymentsShard string
		fromWinCred := false
		useRole := true

		if kernelopts.BuildOptions.IsKernel() || isShellRunner {
			// load via new properties and get config values
			if configMap == nil || len(*configMap) == 0 {
				configMap = &map[string]any{}
				data, err := os.ReadFile("config.yml")
				if err != nil {
					eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("Error reading YAML file: %s", err.Error()), -1)
				}
				err = yaml.Unmarshal(data, configMap)
				if err != nil {
					eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("Error unmarshaling YAML: %s", err.Error()), -1)
				}
			}

			// Unmarshal the YAML data into the map
			if role, ok := (*configMap)["agent_role"].(string); ok {
				appSec := strings.Split(role, ":")
				if len(appSec) != 2 {
					eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "invalid agent role used for drone trcsh agent", 124)
				}
				if len(appSec[0]) > 0 && len(appSec[1]) > 0 {
					azureDeployRole := []string{appSec[0], appSec[1]}
					driverConfigPtr.CoreConfig.TokenCache.AddRole("hivekernel", &azureDeployRole)
				}
			} else {
				useRole = false
				if !isShellRunner {
					driverConfigPtr.CoreConfig.Log.Println("Error reading config value")
				}
			}
			if region, ok := (*configMap)["region"].(string); ok {
				// Override command line region with config provided region here.
				regionPtr = &region
			}
			if addr, ok := (*configMap)["vault_addr"].(string); ok {
				driverConfigPtr.CoreConfig.TokenCache.SetVaultAddress(&addr)
			} else {
				driverConfigPtr.CoreConfig.Log.Println("Error reading config value")
			}
			if env, ok := (*configMap)["agent_env"].(string); ok {
				agentEnv = env
			} else {
				driverConfigPtr.CoreConfig.Log.Println("Error reading config value")
			}
			if deployments, ok := (*configMap)["deployments"].(string); ok {
				deploymentsShard = deployments
			} else {
				driverConfigPtr.CoreConfig.Log.Println("Error reading config value")
			}
		} else {
			if eUtils.IsWindows() {
				agentRole := os.Getenv("AGENT_ROLE")
				if agentRole == "" || agentRole == "UNSET" {
					role, err := wincred.GetGenericCredential("AGENT_ROLE")
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error loading authentication from Credential Manager")
						driverConfigPtr.CoreConfig.Log.Println("Error loading authentication from Credential Manager")
						useRole = false
					} else {
						agentRole := string(role.CredentialBlob)
						fromWinCred = true
						appSec := strings.Split(agentRole, ":")
						if len(appSec) != 2 {
							eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "invalid agent role used from wincred for drone trcsh agent", 124)
						}
						if len(appSec[0]) > 0 && len(appSec[1]) > 0 {
							azureDeployRole := []string{appSec[0], appSec[1]}
							driverConfigPtr.CoreConfig.TokenCache.AddRole("hivekernel", &azureDeployRole)
						}
					}
				} else {
					appSec := strings.Split(agentRole, ":")
					if len(appSec) != 2 {
						eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "invalid agent role used from wincred for drone trcsh agent", 124)
					}
					if len(appSec[0]) > 0 && len(appSec[1]) > 0 {
						azureDeployRole := []string{appSec[0], appSec[1]}
						driverConfigPtr.CoreConfig.TokenCache.AddRole("hivekernel", &azureDeployRole)
					}
				}

			} else {
				agentRole := os.Getenv("AGENT_ROLE")
				if agentRole == "" {
					fmt.Fprintln(os.Stderr, "Error loading authentication from env")
					driverConfigPtr.CoreConfig.Log.Println("Error loading authentication from env")
					useRole = false
				} else {
					appSec := strings.Split(agentRole, ":")
					if len(appSec) != 2 {
						eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "invalid agent role used from wincred for drone trcsh agent", 124)
					}
					if len(appSec[0]) > 0 && len(appSec[1]) > 0 {
						azureDeployRole := []string{appSec[0], appSec[1]}
						driverConfigPtr.CoreConfig.TokenCache.AddRole("hivekernel", &azureDeployRole)
					}
				}
			}
			agentEnv = os.Getenv("AGENT_ENV")
			if len(os.Getenv("VAULT_ADDR")) > 0 {
				vaultAddr := os.Getenv("VAULT_ADDR")
				driverConfigPtr.CoreConfig.TokenCache.SetVaultAddress(&vaultAddr)
			}

			// Replace dev-1 with DEPLOYMENTS-1
			deploymentsKey := "DEPLOYMENTS"
			subDeploymentIndex := strings.Index(*envPtr, "-")
			if subDeploymentIndex != -1 {
				deploymentsKey += (*envPtr)[subDeploymentIndex:]
			}
			deploymentsShard = os.Getenv(deploymentsKey)

			if len(deploymentsShard) == 0 {
				deploymentsShard = os.Getenv(strings.Replace(deploymentsKey, "-", "_", 1))
				if len(deploymentsShard) == 0 {
					eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("drone trcsh requires a %s\n", deploymentsKey), -1)
				}
			}
		}

		if !useRole && !eUtils.IsWindows() && kernelopts.BuildOptions.IsKernel() && !isShellRunner && !driverConfigPtr.CoreConfig.IsEditor {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "drone trcsh requires AGENT_ROLE", -1)
		}

		if len(agentEnv) == 0 {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "drone trcsh requires AGENT_ENV", -1)
		}

		if len(*envPtr) > 0 {
			agentEnv = *envPtr
		}

		if !*tracelessPtr && (strings.HasPrefix(agentEnv, "staging") || strings.HasPrefix(agentEnv, "prod")) {
			prod.SetProd(true)
		}

		if eUtils.RefLength(driverConfigPtr.CoreConfig.TokenCache.VaultAddressPtr) == 0 {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, "drone trcsh requires VAULT_ADDR address.", -1)
		}

		if err := capauth.ValidateVhost(*driverConfigPtr.CoreConfig.TokenCache.VaultAddressPtr, "https://", false, driverConfigPtr.CoreConfig.Log); err != nil {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("drone trcsh requires supported VAULT_ADDR address: %s\n", err.Error()), 124)
		}
		var kernelID int
		var kernelName string = "trcshk"
		if kernelopts.BuildOptions.IsKernel() && !driverConfigPtr.CoreConfig.IsEditor {
			hostname := os.Getenv("HOSTNAME")
			id := 0

			if len(hostname) == 0 {
				driverConfigPtr.CoreConfig.Log.Println("Looking up set entry by host")
				hostOutput, err := os.ReadFile("/etc/hostname")

				if err != nil || len(hostname) == 0 {
					hostOutput, err := exec.Command("hostname").Output()
					if err == nil {
						hostLines := strings.Split(string(hostOutput), "\n")
						for _, hostLine := range hostLines {
							hostLine = strings.TrimSpace(hostLine)
							if len(hostLine) > 0 {
								hostname = hostLine
								break
							}
						}
					}
				} else {
					hostname = string(hostOutput)
				}
			}
			if matches, _ := regexp.MatchString("\\-\\d+$", hostname); matches {
				driverConfigPtr.CoreConfig.Log.Println("Stateful set enabled")

				// <pod>-<pool>
				hostParts := strings.Split(hostname, "-")
				var err error
				id, err = strconv.Atoi(hostParts[1])
				if err != nil {
					id = 0
				}
				kernelID = id
				kernelName = hostParts[0]
				driverConfigPtr.CoreConfig.Log.Printf("Starting Stateful trcshk with set entry id: %d\n", id)
			} else {
				driverConfigPtr.CoreConfig.Log.Printf("Unable to match: %s\n", hostname)
			}
			if id > 0 {
				agentEnv = fmt.Sprintf("%s-%d", agentEnv, id)
			}
			driverConfigPtr.CoreConfig.Log.Printf("Identified as: %s\n", agentEnv)
		}

		if kernelopts.BuildOptions.IsKernel() && !eUtils.IsWindows() {
			if driverConfigPtr != nil && driverConfigPtr.CoreConfig != nil && driverConfigPtr.CoreConfig.IsEditor {
				agentEnv = eUtils.GetEnvBasis(agentEnv)
				fmt.Fprintf(os.Stderr, "Editing for environment %s\n", agentEnv)
			} else {
				fmt.Fprintf(os.Stderr, "Using environment %s for kernel.\n", agentEnv)
			}
		}

		shutdown := make(chan bool)

		if !isShellRunner && !kernelopts.BuildOptions.IsKernel() {
			fmt.Fprintf(os.Stderr, "drone trcsh beginning new agent configuration sequence.\n")
			driverConfigPtr.CoreConfig.Log.Printf("drone trcsh beginning new agent configuration sequence.\n")
		} else {
			gTokenCache = driverConfigPtr.CoreConfig.TokenCache
		}
		// Preload agent synchronization configs...
		trcshDriverConfig, err := TrcshInitConfig(driverConfigPtr,
			agentEnv,
			*regionPtr,
			"",
			true,                               // useMemCache
			kernelopts.BuildOptions.IsKernel(), // outputMemCache
			false,                              // isShell
			nil,
			*tracelessPtr,
			driverConfigPtr.CoreConfig.Log,
		)
		if err != nil {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("drone trcsh agent bootstrap init config failure: %s\n", err.Error()), 124)
		}

		if useRole {
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth..")

			authTokenEnv := agentEnv
			roleEntity := "hivekernel"
			authTokenName := fmt.Sprintf("trcsh_agent_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
			trcshEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

			tokenPtr := new(string)
			if dronePtr != nil {
				trcshDriverConfig.DriverConfig.IsDrone = *dronePtr
			}
			autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, &authTokenName, &tokenPtr, &authTokenEnv, &trcshEnvBasis, &roleEntity, false)
			if autoErr != nil || eUtils.RefLength(trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken(authTokenName)) == 0 {
				eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, fmt.Sprintf("Unable to auth: %s\n", autoErr.Error()), -1)
			}
		}

		var errAgentLoad error
		gAgentConfig, gTrcshConfig, errAgentLoad = capauth.NewAgentConfig(
			trcshDriverConfig.DriverConfig.CoreConfig.TokenCache,
			fmt.Sprintf("trcsh_agent_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis),
			agentEnv,
			deployCtlAcceptRemoteNoTimeout,
			nil,
			true,
			isShellRunner,
			driverConfigPtr.CoreConfig.Log,
			dronePtr)
		if errAgentLoad != nil {
			// check os.env for another token
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("drone trcsh agent bootstrap agent config failure: %s\n", errAgentLoad.Error()), 124)
		}

		if !gTrcshConfig.IsShellRunner {
			driverConfigPtr.CoreConfig.Log.Println("Drone trcsh agent bootstrap successful.")
		}
		if kernelopts.BuildOptions.IsKernel() {
			authTokenEnv := agentEnv
			roleEntity := "bamboo"
			authTokenName := fmt.Sprintf("config_token_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
			trcshEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

			// Pre cache wanted token.
			autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, &authTokenName, nil, &authTokenEnv, &trcshEnvBasis, &roleEntity, false)
			if autoErr != nil {
				eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("trcsh agent bootstrap agent auth failure: %s\n", autoErr.Error()), 124)
			}
		}

		gTokenCache = trcshDriverConfig.DriverConfig.CoreConfig.TokenCache

		if eUtils.IsWindows() {
			if !fromWinCred {
				// migrate token to wincred
				var cred *wincred.GenericCredential
				if useRole {
					cred = wincred.NewGenericCredential("AGENT_ROLE")
					role := driverConfigPtr.CoreConfig.TokenCache.GetRole("hivekernel")
					cred.CredentialBlob = []byte((*role)[0] + ":" + (*role)[1])
					err := cred.Write()
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error migrating updated role: %s\n", err)
						driverConfigPtr.CoreConfig.Log.Printf("Error migrating updated role: %s\n", err)
					} else {
						// delete os.env token
						if os.Getenv("AGENT_TOKEN") != "" {
							command := exec.Command("cmd", "/C", "setx", "/M", "AGENT_TOKEN", "UNSET")
							_, err := command.CombinedOutput()
							if err != nil {
								fmt.Fprintln(os.Stderr, err)
								driverConfigPtr.CoreConfig.Log.Println(err)
							}
						}
						if os.Getenv("AGENT_ROLE") != "" {
							command := exec.Command("cmd", "/C", "setx", "/M", "AGENT_ROLE", "UNSET")
							_, err := command.CombinedOutput()
							if err != nil {
								fmt.Fprintln(os.Stderr, err)
								driverConfigPtr.CoreConfig.Log.Println(err)
							}
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "Error migrating updated role or token: %s\n", err)
					driverConfigPtr.CoreConfig.Log.Printf("Error migrating updated role or token: %s\n", err)
				}
			}
		}

		if !gTrcshConfig.IsShellRunner {
			fmt.Fprintf(os.Stderr, "drone trcsh beginning initialization sequence.\n")
			driverConfigPtr.CoreConfig.Log.Printf("drone trcsh beginning initialization sequence.\n")
		}
		// Initialize deployers.

		// Validate drone sha path
		pluginConfig := make(map[string]any)
		pluginConfig["vaddress"] = *driverConfigPtr.CoreConfig.TokenCache.VaultAddressPtr

		var currentTokenName string
		if isShellRunner {
			currentTokenName = (*configMap)["token_name"].(string)
		} else {
			currentTokenName = fmt.Sprintf("trcsh_agent_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		}
		pluginConfig["tokenptr"] = trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken(currentTokenName)
		pluginConfig["env"] = trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

		if eUtils.IsWindows() {
			pluginConfig["plugin"] = "trcsh.exe"
		} else if kernelopts.BuildOptions.IsKernel() && !isShellRunner {
			pluginConfig["plugin"] = "trcshk"
		} else {
			if isShellRunner {
				pluginConfig["plugin"] = (*configMap)["plugin_name"]
			} else {
				pluginConfig["plugin"] = "trcsh"
			}
		}

		_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig,
			gTokenCache,
			currentTokenName, driverConfigPtr.CoreConfig.Log)
		if err != nil {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("Problem initializing mod: %s\n", err.Error()), 124)
		}
		if vault != nil {
			defer vault.Close()
		}

		isValid, err := trcshauth.ValidateTrcshPathSha(mod, pluginConfig, driverConfigPtr.CoreConfig.Log)
		if err != nil || !isValid {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("Error obtaining authorization components: %s\n", err.Error()), 124)
		}

		if kernelPluginHandler == nil && !isShellRunner {
			kernelPluginHandler = hive.InitKernel(fmt.Sprintf("%s-%d", kernelName, kernelID))
			kernelPluginHandler.ConfigContext.Log = driverConfigPtr.CoreConfig.Log
			go kernelPluginHandler.DynamicReloader(trcshDriverConfig.DriverConfig)
		}

		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Completed bootstrapping and continuing to initialize services.")

		deploymentShards := strings.Split(deploymentsShard, ",")

		// This is a tad more complex but will scale more nicely.
		deploymentShardsSet := map[string]struct{}{}
		for _, str := range deploymentShards {
			deploymentShardsSet[str] = struct{}{}
		}

		deployablePlugins, err := deployutil.GetDeployers(kernelPluginHandler, trcshDriverConfig, deploymentShardsSet, dronePtr, &isShellRunner)
		if err != nil {
			eUtils.LogSyncAndExit(driverConfigPtr.CoreConfig.Log, fmt.Sprintf("drone trcsh agent bootstrap get deployers failure: %s\n", err.Error()), 124)
		}
		pluginDeployments := []*map[string]interface{}{}

		if eUtils.IsWindows() || kernelopts.BuildOptions.IsKernel() {
			for _, deployablePluginConfig := range deployablePlugins {
				if deployablePluginConfig != nil {
					// Extract deployment name from the config map
					if trcPlugin, ok := (*deployablePluginConfig)["trcplugin"]; ok {
						if deploymentName, isString := trcPlugin.(string); isString {
							if _, ok := deploymentShardsSet[deploymentName]; ok {
								pluginDeployments = append(pluginDeployments, deployablePluginConfig)
								if kernelPluginHandler != nil {
									kernelPluginHandler.AddKernelPlugin(deploymentName, trcshDriverConfig.DriverConfig, deployablePluginConfig)
								}
							}
						}
					}
				}
			}
			if kernelPluginHandler != nil {
				kernelPluginHandler.InitPluginStatus(trcshDriverConfig.DriverConfig)
			}
		}

		// Build deployment names list for legacy compatibility
		deployments := []string{}
		for _, pluginConfig := range pluginDeployments {
			if trcPlugin, ok := (*pluginConfig)["trcplugin"]; ok {
				if deploymentName, isString := trcPlugin.(string); isString {
					deployments = append(deployments, deploymentName)
				}
			}
		}

		deploymentsCDL := strings.Join(deployments, ",")
		gAgentConfig.Deployments = &deploymentsCDL

		deployopts.BuildOptions.InitSupportedDeployers(deployments)

		if len(pluginDeployments) == 0 {
			fmt.Fprintln(os.Stderr, "No valid deployments for trcshell, entering hibernate mode.")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("No valid deployments for trcshell, entering hibernate mode.")
			hibernate := make(chan bool)
			hibernate <- true
		}

		if kernelopts.BuildOptions.IsKernel() && kernelPluginHandler != nil {
			go func(kpH *hive.PluginHandler, dc *config.DriverConfig) {
				kpH.HandleChat(dc)
			}(kernelPluginHandler, trcshDriverConfig.DriverConfig)
		}

		// Prioritize healthcheck deployment - start it first
		healthcheckIdx := -1
		for i, deploymentConfig := range pluginDeployments {
			if trcPlugin, ok := (*deploymentConfig)["trcplugin"]; ok {
				if deploymentName, isString := trcPlugin.(string); isString && deploymentName == "healthcheck" {
					healthcheckIdx = i
					EnableDeployer(driverConfigPtr,
						*gAgentConfig.Env,
						*regionPtr,
						"",
						*trcPathPtr,
						true,
						kernelopts.BuildOptions.IsKernel(),
						dronePtr,
						deploymentConfig,
						tracelessPtr,
						projectServicePtr)
					driverConfigPtr.CoreConfig.Log.Println("Healthcheck deployer started, waiting 5 seconds before starting other deployers...")
					for {
						if kernelPluginHandler != nil && kernelPluginHandler.Services != nil {
							if healthcheckService, ok := (*kernelPluginHandler.Services)["healthcheck"]; ok {
								if healthcheckService.State == 1 {
									break
								}
							}
						}
						time.Sleep(1 * time.Second)
					}
					break
				}
			}
		}

		// Remove healthcheck from pluginDeployments list if it was found
		if healthcheckIdx >= 0 {
			pluginDeployments = append(pluginDeployments[:healthcheckIdx], pluginDeployments[healthcheckIdx+1:]...)
		}

		for _, deploymentConfig := range pluginDeployments {
			if kernelopts.BuildOptions.IsKernel() {
				go func(dcPtr *config.DriverConfig,
					env string,
					region string,
					trcPath string,
					outputMemCache bool,
					dronePtr *bool,
					trcLessPtr *bool,
					projectService *string,
					allPluginDeployments []*map[string]interface{},
				) {
					for {
						deploy := <-*kernelPluginHandler.KernelCtx.DeployRestartChan
						dcPtr.CoreConfig.Log.Printf("Restarting deploy for %s.\n", deploy)
						// Find the deployment config for this deploy name
						var deployConfig *map[string]interface{}
						for _, pc := range allPluginDeployments {
							if trcPlugin, ok := (*pc)["trcplugin"]; ok {
								if deploymentName, isString := trcPlugin.(string); isString && deploymentName == deploy {
									deployConfig = pc
									break
								}
							}
						}
						if deployConfig != nil {
							go EnableDeployer(dcPtr,
								env,
								region,
								"",
								trcPath,
								true, // useMemCache
								outputMemCache,
								dronePtr,
								deployConfig,
								trcLessPtr,
								projectService)
						}
					}
				}(driverConfigPtr,
					*gAgentConfig.Env,
					*regionPtr,
					*trcPathPtr,
					kernelopts.BuildOptions.IsKernel(), // outputMemCache
					dronePtr,
					tracelessPtr,
					projectServicePtr,
					pluginDeployments)
			}
			EnableDeployer(driverConfigPtr,
				*gAgentConfig.Env,
				*regionPtr,
				"",
				*trcPathPtr,
				true,           // useMemCache
				!isShellRunner, // outputMemCache
				dronePtr,
				deploymentConfig,
				tracelessPtr,
				projectServicePtr)
		}

		<-shutdown
	}
	return nil
}

var (
	interruptChan chan os.Signal = make(chan os.Signal, 5)
	//	twoHundredMilliInterruptTicker *time.Ticker   = time.NewTicker(200 * time.Millisecond)
	//	secondInterruptTicker          *time.Ticker   = time.NewTicker(time.Second)
	multiSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 3)
	//	fiveSecondInterruptTicker      *time.Ticker   = time.NewTicker(time.Second * 5)
	fifteenSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 5)
	thirtySecondInterruptTicker  *time.Ticker = time.NewTicker(time.Second * 5)
)

func acceptInterruptFun(featherCtx *cap.FeatherContext, tickerContinue *time.Ticker, tickerBreak *time.Ticker, tickerInterrupt *time.Ticker) (bool, error) {
	result := false
	var resultError error = nil
	select {
	case <-tickerContinue.C:
		// don't break... continue...
		result = false
		resultError = nil
	case <-tickerBreak.C:
		// break and continue
		result = true
		resultError = nil
	case <-tickerInterrupt.C:
		// full stop
		result = true
		resultError = errors.New(YOU_SHALL_NOT_PASS)
	}
	if len(featherCtx.InterruptChan) > 0 {
		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
		eUtils.LogSyncAndExit(featherCtx.Log, "Accept Interrupted", 128)
	}
	return result, resultError
}

func acceptInterruptNoTimeoutFun(featherCtx *cap.FeatherContext, tickerContinue *time.Ticker) (bool, error) {
	result := false
	var resultError error = nil
	<-tickerContinue.C
	// don't break... continue...
	result = false
	resultError = nil
	if len(featherCtx.InterruptChan) > 0 {
		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
		eUtils.LogSyncAndExit(featherCtx.Log, "Accept Interrupted No timeout", 128)
	}
	return result, resultError
}

// func interruptFun(featherCtx *cap.FeatherContext, tickerInterrupt *time.Ticker) {
// 	<-tickerInterrupt.C
// 	if len(featherCtx.InterruptChan) > 0 {
// 		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
// 		eUtils.LogSyncAndExit(featherCtx.Log, "Interrupt Interrupted", 128)
// 	}
// }

// acceptRemote - hook for instrumenting
func acceptRemote(featherCtx *cap.FeatherContext, mode int, _ string) (bool, error) {
	if mode == cap.FEATHER_CTL {
		return acceptInterruptFun(featherCtx, multiSecondInterruptTicker, fifteenSecondInterruptTicker, thirtySecondInterruptTicker)
	}
	return true, nil
}

func featherCtlCb(featherCtx *cap.FeatherContext, agentName string) error {
	if gAgentConfig == nil {
		return errors.New("incorrect agent initialization")
	}

	if featherCtx == nil {
		return errors.New("incorrect feathering")
	}

	// Initialize supoorted deployers.
	deployopts.BuildOptions.InitSupportedDeployers([]string{agentName})

	if sessionIDentifier, ok := deployopts.BuildOptions.GetEncodedDeployerId(agentName, *featherCtx.Env); ok {
		featherCtx.SessionIdentifier = &sessionIDentifier
		featherCtx.Log.Printf("Starting deploy ctl session: %s\n", sessionIDentifier)
		captiplib.FeatherCtl(featherCtx, deployerCtlEmote)
	} else {
		eUtils.LogSyncAndExit(featherCtx.Log, fmt.Sprintf("Unsupported agent: %s\n", agentName), 123)
	}

	return nil
}

func roleBasedRunner(
	region string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	control string,
	_ []string,
	deployArgLines []string,
	configCount *int,
) error {
	*configCount -= 1
	if trcshDriverConfig.DriverConfig.CoreConfig.CurrentRoleEntityPtr == nil {
		currentRoleEntityPtr := new(string)
		*currentRoleEntityPtr = "config.yml" // Chewbacca: Why?!?!
		trcshDriverConfig.DriverConfig.CoreConfig.CurrentRoleEntityPtr = currentRoleEntityPtr
	}
	trcshDriverConfig.DriverConfig.FileFilter = nil
	trcshDriverConfig.DriverConfig.CoreConfig.WantCerts = false
	trcshDriverConfig.DriverConfig.IsShellSubProcess = true
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner init: %s\n", control)

	if trcshDriverConfig.DriverConfig.DeploymentConfig != nil {
		if trcDeployRoot, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcdeployroot"]; ok {
			trcshDriverConfig.DriverConfig.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
			trcshDriverConfig.DriverConfig.EndDir = trcDeployRoot.(string)
		}
	}
	if trcshDriverConfig.DriverConfig.DeploymentConfig == nil || len(*trcshDriverConfig.DriverConfig.DeploymentConfig) == 0 {
		trcshDriverConfig.DriverConfig.StartDir = []string{"trc_templates"}
		trcshDriverConfig.DriverConfig.EndDir = "."
	}
	tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	envDefaultPtr := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	var err error
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner started: %s\n", control)

	switch control {
	case "trcplgtool":
		envDefaultPtr = trcshDriverConfig.DriverConfig.CoreConfig.Env
		if gTrcshConfig.IsShellRunner || trcshDriverConfig.DriverConfig.IsDrone {
			// Drone and shell do not need special access.
			tokenName = *trcshDriverConfig.DriverConfig.CoreConfig.GetCurrentToken("config_token_%s")
		} else {
			tokenName = "config_token_pluginany"
		}
		if kernelPluginHandler != nil {
			err = trcplgtoolbase.CommonMain(&envDefaultPtr, &gTrcshConfig.EnvContext, &tokenName, &region, nil, deployArgLines, trcshDriverConfig, kernelPluginHandler)
		} else {
			err = trcplgtoolbase.CommonMain(&envDefaultPtr, &gTrcshConfig.EnvContext, &tokenName, &region, nil, deployArgLines, trcshDriverConfig)
		}
	case "trcconfig":
		roleEntityPtr := new(string)
		*roleEntityPtr = "configrole"
		trcshDriverConfig.DriverConfig.CoreConfig.CurrentRoleEntityPtr = roleEntityPtr

		if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" || (prod.IsProd() && prod.IsStagingProd(trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)) ||
			trcshDriverConfig.DriverConfig.CoreConfig.Env == "itdev" || (prod.IsProd() && prod.IsStagingProd(trcshDriverConfig.DriverConfig.CoreConfig.Env)) {
			if !kernelopts.BuildOptions.IsKernel() {
				trcshDriverConfig.DriverConfig.OutputMemCache = false
			}
			// itdev, staging, and prod always key off TRC_ENV stored in trcshDriverConfig.DriverConfig.CoreConfig.Env.
			envDefaultPtr = trcshDriverConfig.DriverConfig.CoreConfig.Env
			tokenName = "config_token_" + eUtils.GetEnvBasis(trcshDriverConfig.DriverConfig.CoreConfig.Env)
		}
		if trcshDriverConfig.DriverConfig.IsDrone && eUtils.IsWindows() {
			// Can this be enabled without disrupting the kernel on linux?
			trcshDriverConfig.DriverConfig.OutputMemCache = false
		}
		err = trcconfigbase.CommonMain(&envDefaultPtr, &gTrcshConfig.EnvContext, &tokenName, &region, nil, deployArgLines, trcshDriverConfig.DriverConfig)
	case "trcsub":
		trcshDriverConfig.DriverConfig.EndDir = trcshDriverConfig.DriverConfig.EndDir + "/trc_templates"
		err = trcsubbase.CommonMain(&envDefaultPtr, &gTrcshConfig.EnvContext, &tokenName, nil, deployArgLines, trcshDriverConfig.DriverConfig)
	}
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner complete: %s\n", control)

	return err
}

func processPluginCmds(trcKubeDeploymentConfig **kube.TrcKubeConfig,
	onceKubeInit *sync.Once,
	PipeOS trcshio.TrcshReadWriteCloser,
	env string,
	region string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	control string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int,
) {
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Processing control: %s\n", control)

	switch control {
	case "trccertinit":
		if prod.IsProd() {
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "trccertinit unsupported in production\n", 125)
		}
		tokenName := fmt.Sprintf("vault_pub_token_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		currentRoleEntityPtr := new(string)
		*currentRoleEntityPtr = "pubrole"
		trcshDriverConfig.DriverConfig.CoreConfig.CurrentRoleEntityPtr = currentRoleEntityPtr
		trcshDriverConfig.DriverConfig.IsShellSubProcess = true
		trcshDriverConfig.DriverConfig.CoreConfig.WantCerts = true
		if trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetRole("pub") == nil ||
			trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetRole("pubrole") == nil {
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "Missing required certification auth components\n", 125)
		}
		pubEnv := env

		trcinitbase.CommonMain(&pubEnv,
			&gTrcshConfig.EnvContext,
			&tokenName,
			&trcshDriverConfig.DriverConfig.CoreConfig.WantCerts,
			nil,
			deployArgLines,
			trcshDriverConfig.DriverConfig)
	case "trcpub":
		tokenName := fmt.Sprintf("vault_pub_token_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		roleEntityPtr := new(string)
		*roleEntityPtr = "pubrole"
		trcshDriverConfig.DriverConfig.CoreConfig.CurrentRoleEntityPtr = roleEntityPtr
		trcshDriverConfig.DriverConfig.IsShellSubProcess = true
		if trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetRole("pub") == nil {
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "Missing required pub auth components\n", 125)
		}
		pubEnv := env
		trcpubbase.CommonMain(&pubEnv, &gTrcshConfig.EnvContext, &tokenName, nil, deployArgLines, trcshDriverConfig.DriverConfig)
	case "trcconfig":
		err := roleBasedRunner(region, trcshDriverConfig, control, argsOrig, deployArgLines, configCount)
		if err != nil {
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, fmt.Sprintf("trcconfig - unexpected failure: %s", err.Error()), 1)
		}
	case "trcplgtool":
		// Utilize elevated CToken to perform certifications if asked.
		trcshDriverConfig.FeatherCtlCb = featherCtlCb
		if gAgentConfig == nil {

			var errAgentLoad error
			if gTrcshConfig == nil || !gTrcshConfig.IsValid(trcshDriverConfig, gAgentConfig) || eUtils.RefLength(gTrcshConfig.TokenCache.GetToken("config_token_pluginany")) == 0 {
				// Chewbacca: Consider removing as this should have already
				// been done earlier in the process.
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Unexpected invalid trcshConfig.  Attempting recovery.")
				retries := 0
				for {
					if gTrcshConfig == nil || !gTrcshConfig.IsValid(trcshDriverConfig, gAgentConfig) {
						var err error
						// Loop until we have something usable...
						gTrcshConfig, err = trcshauth.TrcshAuth(nil, gAgentConfig, trcshDriverConfig)
						retries = retries + 1
						if err != nil {
							trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf(".")
							time.Sleep(time.Second)
							if trcshDriverConfig.DriverConfig.CoreConfig.IsShell && retries >= 7 {
								eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "pipeline auth setup failure.  Cannot continue.\n", 124)
							}
							continue
						} else {
							time.Sleep(time.Second)
						}
						if trcshDriverConfig.DriverConfig.CoreConfig.IsShell && retries >= 7 {
							eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "pipeline auth setup partial failure.  Cannot continue.\n", 124)
						}
						trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth re-loaded %s\n", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
					} else {
						break
					}
				}
			}
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Reloading agent configs for control: %s\n", control)

			// Prepare the configuration triggering mechanism.
			// Bootstrap deployment is replaced during callback with the agent name.
			gAgentConfig, _, errAgentLoad = capauth.NewAgentConfig(
				gTrcshConfig.TokenCache,
				"config_token_pluginany",
				env,
				deployCtlAcceptRemote,
				deployCtlInterrupted,
				false,
				gTrcshConfig.IsShellRunner,
				trcshDriverConfig.DriverConfig.CoreConfig.Log)
			if errAgentLoad != nil {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Permissions failure.  Incorrect deployment\n")
				eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "Permissions failure.  Incorrect deployment\n", 126)
			}
			if gAgentConfig.FeatherContext == nil {
				fmt.Fprintf(os.Stderr, "Warning!  Permissions failure.  Incorrect feathering\n")
			}
			gAgentConfig.InterruptHandlerFunc = deployCtlInterrupted
		}
		if !trcshDriverConfig.DriverConfig.CoreConfig.IsEditor {
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Feather ctl init for control: %s\n", control)
			trcshDriverConfig.FeatherCtx = captiplib.FeatherCtlInit(interruptChan,
				gAgentConfig.LocalHostAddr,
				gAgentConfig.EncryptPass,
				gAgentConfig.EncryptSalt,
				gAgentConfig.HostAddr,
				gAgentConfig.HandshakeCode,
				new(string),
				&env,
				deployCtlAcceptRemote,
				deployCtlInterrupted)
			if trcshDriverConfig.DriverConfig.CoreConfig.Log != nil {
				trcshDriverConfig.FeatherCtx.Log = trcshDriverConfig.DriverConfig.CoreConfig.Log
			}
		}

		trcshDriverConfig.DriverConfig.CoreConfig.TokenCache = gTrcshConfig.TokenCache
		err := roleBasedRunner(region, trcshDriverConfig, control, argsOrig, deployArgLines, configCount)
		if err != nil {
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, fmt.Sprintf("trcplgtool - unexpected failure: %s", err.Error()), 1)
		}

	case "kubectl":
		onceKubeInit.Do(func() {
			var kubeInitErr error
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Setting up kube config")
			*trcKubeDeploymentConfig, kubeInitErr = kube.InitTrcKubeConfig(gTrcshConfig, trcshDriverConfig.DriverConfig.CoreConfig)
			if kubeInitErr != nil {
				fmt.Fprintln(os.Stderr, kubeInitErr)
				return
			}
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Setting kube config setup complete")
		})
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Preparing for kubectl")
		(*(*trcKubeDeploymentConfig)).PipeOS = PipeOS

		kubectlErrChan := make(chan error, 1)

		go func(dConfig *config.DriverConfig) {
			dConfig.CoreConfig.Log.Println("Executing kubectl")
			kubectlErrChan <- kube.KubeCtl(*trcKubeDeploymentConfig, dConfig)
		}(trcshDriverConfig.DriverConfig)

		select {
		case <-time.After(15 * time.Second):
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "Kubernetes connection stalled or timed out.  Possible kubernetes ip change", -1)
		case kubeErr := <-kubectlErrChan:
			if kubeErr != nil {
				eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, kubeErr.Error(), -1)
			}
		}
	}
}

func processDroneCmds(_ *kube.TrcKubeConfig,
	_ *sync.Once,
	_ trcshio.TrcshReadWriteCloser,
	region string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	control string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int,
) error {
	err := roleBasedRunner(region, trcshDriverConfig, control, argsOrig, deployArgLines, configCount)
	return err
}

// ProcessDeploy executes the deployment process for the given environment, region, deployment name, script path, project service, and drone flag.
//
// Parameters:
//
//	featherCtx: Current FeatherContext.
//	trcshDriverConfig: Driver configuration for trcsh.
//	deployment: Name of deployment.
//	trcPath: Path to the current deployment script.
//	projectService: Project service name.
//	dronePtr: Pointer to drone flag.
func ProcessDeploy(featherCtx *cap.FeatherContext,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	trcPath string,
	projectService string,
	dronePtr *bool,
) {
	// Extract deployment name from config
	var deployment string
	if trcshDriverConfig.DriverConfig.DeploymentConfig != nil {
		if trcPlugin, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcplugin"]; ok {
			if deploymentName, isString := trcPlugin.(string); isString {
				deployment = deploymentName
			}
		}
	}
	pwd, _ := os.Getwd()
	var content []byte

	if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" {
		trcshDriverConfig.DriverConfig.OutputMemCache = false
	}
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Logging initialized for env:%s\n", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)

	// Always try to get the vault address using TrcshVAddress regardless of IsShell value
	var vAuthErr error
	vaultAddress, vAuthErr := trcshauth.TrcshVAddress(featherCtx, gAgentConfig, trcshDriverConfig)
	// Chewbacca: scrub before checkin
	// This data is generated by TrcshAuth
	// var vaultAddress *string
	// vAddress := os.Getenv("VAULT_ADDR")
	// vaultAddress = &vAddress
	// vAuthErr = nil
	//	Chewbacca: end scrub
	if vAuthErr != nil || vaultAddress == nil || len(*vaultAddress) == 0 {
		fmt.Fprintln(os.Stderr, "Auth phase 0 failure - failed to get vault address")
		errMsg := ""
		if vAuthErr != nil {
			errMsg = fmt.Sprintf("Error: %s\n", vAuthErr.Error())
		} else {
			errMsg = "Error: TrcshVAddress returned empty or nil address\n"
		}
		eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, errMsg, -1)
	}
	trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.SetVaultAddress(vaultAddress)

	// Chewbacca: scrub before checkin
	// This data is generated by TrcshAuth
	configRole := os.Getenv("CONFIG_ROLE")
	pubRole := os.Getenv("PUB_ROLE")
	pluginAny := os.Getenv("PLUGIN_ANY")
	fileBytes, _ := os.ReadFile("")
	kc := base64.StdEncoding.EncodeToString(fileBytes)
	gTrcshConfig = &capauth.TrcShConfig{
		Env:           "dev",
		EnvContext:    "dev",
		TokenCache:    trcshDriverConfig.DriverConfig.CoreConfig.TokenCache,
		KubeConfigPtr: &kc,
	}
	vAddr := os.Getenv("VAULT_ADDR")
	trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.SetVaultAddress(&vAddr)
	trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.AddToken("config_token_pluginany", &pluginAny)
	trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.AddRoleStr("bamboo", &configRole)
	trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.AddRoleStr("pub", &pubRole)
	// Chewbacca: end scrub

	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth..")
	trcshEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	deployTokenPtr := new(string)
	authTokenEnv := "hivekernel"
	currentRoleEntity := "deployauth"
	if gAgentConfig != nil && gAgentConfig.AgentToken != nil {
		deployTokenPtr = gAgentConfig.AgentToken
		currentRoleEntity = "none"
	}
	authTokenName := "vault_token_azuredeploy"
	autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, &authTokenName, &deployTokenPtr, &authTokenEnv, &trcshEnvBasis, &currentRoleEntity, false)
	if autoErr != nil || eUtils.RefLength(trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken("vault_token_azuredeploy")) == 0 {
		eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, fmt.Sprintf("Unable to auth: %s\n", autoErr.Error()), -1)
	}

	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Bootstrap..")
	var err error
	retries := 0
	for {
		if gTrcshConfig == nil || !gTrcshConfig.IsValid(trcshDriverConfig, gAgentConfig) {
			// Loop until we have something usable...
			gTrcshConfig, err = trcshauth.TrcshAuth(featherCtx, gAgentConfig, trcshDriverConfig)
			if err != nil {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf(".")
				time.Sleep(time.Second)
				retries = retries + 1
				if trcshDriverConfig.DriverConfig.CoreConfig.IsShell && retries >= 7 {
					eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "pipeline auth setup failure.  Cannot continue.\n", 124)
				}
				continue
			} else {
				if retries > 0 {
					time.Sleep(time.Second)
				}
			}
			retries = retries + 1
			if trcshDriverConfig.DriverConfig.CoreConfig.IsShell && retries >= 7 {
				eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "pipeline auth setup partial failure.  Cannot continue.\n", 124)
			}
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth re-loaded %s\n", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		} else {
			break
		}
	}
	// Chewbacca: Begin dbg comment
	mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

	if len(mergedEnvBasis) == 0 {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil {
			mergedEnvBasis = eUtils.GetEnvBasis(gTrcshConfig.EnvContext)
		}
	}

	// End dbg comment
	if trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Session Authorized")
	} else {
		fmt.Fprintln(os.Stderr, "Session Authorized")
	}

	// Set up a separate deployer config for the deployer process.
	var deployerDriverConfig config.DriverConfig
	deployerDriverConfig.CoreConfig = trcshDriverConfig.DriverConfig.CoreConfig
	deployerDriverConfig.IsDrone = trcshDriverConfig.DriverConfig.IsDrone
	deployerDriverConfig.SubOutputMemCache = true
	deployerDriverConfig.OutputMemCache = true
	deployerDriverConfig.ReadMemCache = true
	deployerDriverConfig.ZeroConfig = true
	deployerDriverConfig.MemFs = trcshmemfs.NewTrcshMemFs()
	deployerDriverConfig.DeploymentConfig = trcshDriverConfig.DriverConfig.DeploymentConfig

	if trcshDriverConfig.DriverConfig.CoreConfig.IsShell || (trcshDriverConfig.DriverConfig.DeploymentConfig != nil && (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"] != nil && ((*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"].(string) == "trcshpluginservice" || (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"].(string) == "trcflowpluginservice")) {
		// Generate trc code...
		deployerDriverConfig.CoreConfig.Log.Println("Preload setup")
		if trcshDriverConfig.DriverConfig.DeploymentConfig != nil {
			if pjService, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcprojectservice"]; ok {
				if pjServiceStr, isString := pjService.(string); isString {
					projectService = pjServiceStr
				} else {
					deployerDriverConfig.CoreConfig.Log.Printf("Kernel Missing plugin component project service: %s.\n", deployment)
					return
				}
			} else {
				deployerDriverConfig.CoreConfig.Log.Printf("Kernel Missing plugin component project service: %s.\n", deployment)
				return
			}

			if trcBootstrap, ok := (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcbootstrap"]; ok {
				if bootstrapStr, isString := trcBootstrap.(string); isString && strings.Contains(bootstrapStr, "/deploy/") {
					trcPath = bootstrapStr
				} else {
					deployerDriverConfig.CoreConfig.Log.Printf("Plugin %s missing plugin component bootstrap.\n", deployment)
					return
				}
			} else {
				deployerDriverConfig.CoreConfig.Log.Printf("Plugin %s missing plugin component bootstrap.\n", deployment)
				return
			}
		}

		trcPathParts := strings.Split(trcPath, "/")
		deployerDriverConfig.FileFilter = []string{trcPathParts[len(trcPathParts)-1]}

		if projectService != "" {
			deployerDriverConfig.CoreConfig.Log.Println("Trcsh - Attempting to fetch templates from provided projectServicePtr: " + projectService)
			err := deployutil.MountPluginFileSystem(&deployerDriverConfig, trcPath, projectService)
			if err != nil {
				deployerDriverConfig.CoreConfig.Log.Printf("Trcsh - Failed to fetch template using projectServicePtr: %s", err.Error())
				fmt.Fprintln(os.Stderr, "Trcsh - Failed to fetch template using projectServicePtr. "+err.Error())
				return
			}
			deployerDriverConfig.ServicesWanted = strings.Split(projectService, ",")
			// Also need this later on for running scripts, within the deployment.
			trcshDriverConfig.DriverConfig.ServicesWanted = deployerDriverConfig.ServicesWanted
		}

		deployerDriverConfig.OutputMemCache = true
		deployerDriverConfig.StartDir = []string{"trc_templates"}
		deployerDriverConfig.EndDir = "."
		deployerDriverConfig.CoreConfig.Log.Printf("Preloading path %s env %s\n", trcPath, trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		region := ""
		if len(deployerDriverConfig.CoreConfig.Regions) > 0 {
			region = deployerDriverConfig.CoreConfig.Regions[0]
		}

		envConfig := deployerDriverConfig.CoreConfig.EnvBasis
		if strings.Contains(deployerDriverConfig.CoreConfig.Env, "-") {
			envConfig = deployerDriverConfig.CoreConfig.Env
		}
		tokenNamePtr := deployerDriverConfig.CoreConfig.GetCurrentToken("config_token_%s")
		configErr := trcconfigbase.CommonMain(&envConfig, &mergedEnvBasis, tokenNamePtr, &region, nil, []string{"trcsh"}, &deployerDriverConfig)
		if configErr != nil {
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, fmt.Sprintf("Preload failed.  Couldn't find required resource: %s", configErr.Error()), 123)
		}

		var memFile trcshio.TrcshReadWriteCloser
		var memFileErr error
		if memFile, memFileErr = deployerDriverConfig.MemFs.Open(trcPath); memFileErr == nil {
			// Read the generated .trc code...
			buf := bytes.NewBuffer(nil)
			io.Copy(buf, memFile) // Error handling elided for brevity.
			content = buf.Bytes()
			deployerDriverConfig.MemFs.Remove(trcPath)
			deployerDriverConfig.MemFs.ClearCache("/trc_templates")
		} else {
			if strings.HasPrefix(trcPath, "./") {
				trcPath = strings.TrimLeft(trcPath, "./")
			}
			if memFile, memFileErr = deployerDriverConfig.MemFs.Open(trcPath); memFileErr == nil {
				// Read the generated .trc code...
				buf := bytes.NewBuffer(nil)
				io.Copy(buf, memFile) // Error handling elided for brevity.
				content = buf.Bytes()
				deployerDriverConfig.MemFs.Remove(trcPath)
				deployerDriverConfig.MemFs.ClearCache("/trc_templates")
			} else {
				if strings.HasPrefix(trcPath, "./") {
					trcPath = strings.TrimLeft(trcPath, "./")
				}

				// TODO: Move this out into its own function
				fmt.Fprintln(os.Stderr, "Trcsh - Error could not find "+trcPath+" for deployment instructions..")
			}
		}

		if !kernelopts.BuildOptions.IsKernel() {
			// Ensure trcconfig pulls templates from file system for builds and releases.
			trcshDriverConfig.DriverConfig.ReadMemCache = false
		}

		if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" ||
			(!kernelopts.BuildOptions.IsKernel() && (prod.IsProd() && prod.IsStagingProd(trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis))) {
			trcshDriverConfig.DriverConfig.OutputMemCache = false
			trcshDriverConfig.DriverConfig.ReadMemCache = false
			trcshDriverConfig.DriverConfig.SubOutputMemCache = false
		}
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Processing trcshell")
	} else {
		if !strings.Contains(pwd, "TrcDeploy") || trcshDriverConfig.DriverConfig.DeploymentConfig == nil || len(*trcshDriverConfig.DriverConfig.DeploymentConfig) == 0 {
			fmt.Fprintln(os.Stderr, "Processing manual trcshell")
			if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" {
				content, err = os.ReadFile(pwd + "/deploy/buildtest.trc")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Trcsh - Error could not find /deploy/buildtest.trc for deployment instructions")
				}
			} else {
				content, err = os.ReadFile(pwd + "/deploy/deploy.trc")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Trcsh - Error could not find "+pwd+"/deploy/deploy.trc for deployment instructions")
					trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Trcsh - Error could not find %s/deploy/deploy.trc for deployment instructions", pwd)
				}
			}
		}
	}

collaboratorReRun:
	if featherCtx != nil && content == nil && trcshDriverConfig.DriverConfig.DeploymentConfig != nil && (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"] != nil && (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trctype"].(string) == "trcshservice" {
		// Start with a clean cache always.
		if trcshDriverConfig.DriverConfig != nil && trcshDriverConfig.DriverConfig.MemFs != nil {
			trcshDriverConfig.DriverConfig.MemFs.ClearCache(".")
		}

		// featherCtx initialization is delayed for the self contained deployments (kubernetes, etc...)
		for {
			if atomic.LoadInt64(&featherCtx.RunState) == cap.RESETTING {
				break
			} else {
				acceptRemote(featherCtx, cap.FEATHER_CTL, "")
			}
		}

		content, err = deployutil.LoadPluginDeploymentScript(trcshDriverConfig, gTrcshConfig, pwd)
		if err != nil {
			if trcshDriverConfig.DriverConfig.DeploymentConfig != nil {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Failure to load deployment: %s\n", (*trcshDriverConfig.DriverConfig.DeploymentConfig)["trcplugin"])
			} else {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Failure to load deployment: <unknown>\n")
			}
			time.Sleep(time.Minute)
			content = nil
			goto collaboratorReRun
		}
	}

	deployArgLines := strings.Split(string(content), "\n")
	configCount := strings.Count(string(content), "trcconfig") // Uses this to close result channel on last run.
	argsOrig := os.Args

	var trcKubeDeploymentConfig *kube.TrcKubeConfig
	var onceKubeInit sync.Once
	var PipeOS trcshio.TrcshReadWriteCloser

	for _, deployPipeline := range deployArgLines {
		deployPipeline = strings.TrimLeft(deployPipeline, " \t\r\n")
		if strings.HasPrefix(deployPipeline, "#") || deployPipeline == "" {
			continue
		}
		// Print current process line.
		if trcshDriverConfig.DriverConfig.CoreConfig.IsEditor {
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println(deployPipeline)
		} else {
			fmt.Fprintln(os.Stderr, deployPipeline)
		}
		deployPipeSplit := strings.Split(deployPipeline, "|")

		if PipeOS, err = trcshDriverConfig.DriverConfig.MemFs.Create("io/STDIO"); err != nil {
			eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "Failure to open io stream.", -1)
		}

		for _, deployLine := range deployPipeSplit {
			trcshDriverConfig.DriverConfig.IsShellSubProcess = false
			os.Args = argsOrig
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) // Reset flag parse to allow more toolset calls.

			deployLine = strings.TrimSpace(deployLine)
			deployArgs := strings.Split(deployLine, " ")
			control := deployArgs[0]
			if len(deployArgs) > 1 {
				envArgIndex := -1

				// Supported parameters.
				for dIndex, dArgs := range deployArgs {
					if strings.HasPrefix(dArgs, "-env=") {
						envArgIndex = dIndex
						continue
					}
				}

				if envArgIndex != -1 {
					var tempArgs []string
					if len(deployArgs) > envArgIndex+1 {
						tempArgs = deployArgs[envArgIndex+1:]
					}
					deployArgs = deployArgs[1:envArgIndex]
					if len(tempArgs) > 0 {
						deployArgs = append(deployArgs, tempArgs...)
					}
				} else {
					if control != "kubectl" {
						deployArgs = deployArgs[1:]
					}
				}
				if control != "kubectl" {
					os.Args = append(os.Args, deployArgs...)
				} else {
					os.Args = deployArgs
				}
			}
			if *dronePtr {
				// Log for traceability.
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Println(deployLine)
				region := ""
				if len(trcshDriverConfig.DriverConfig.CoreConfig.Regions) > 0 {
					region = trcshDriverConfig.DriverConfig.CoreConfig.Regions[0]
				}
				trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- deployLine
				err := processDroneCmds(
					trcKubeDeploymentConfig,
					&onceKubeInit,
					PipeOS,
					region,
					trcshDriverConfig,
					control,
					argsOrig,
					strings.Split(deployLine, " "),
					&configCount)
				if err != nil {
					if strings.Contains(err.Error(), "Forbidden") {
						// Critical agent setup error.
						eUtils.LogSyncAndExit(trcshDriverConfig.DriverConfig.CoreConfig.Log, "Critical agent setup error.", -1)
					}
					errMessage := err.Error()
					errMessageFiltered := strings.ReplaceAll(errMessage, ":", "-")
					deliverableMsg := fmt.Sprintf("%s encountered errors - %s\n", deployLine, errMessageFiltered)
					go func(dMesg string) {
						trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- dMesg
						closeCleanupMessaging(trcshDriverConfig)
					}(deliverableMsg)

					content = nil
					goto collaboratorReRun
				}
			} else {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Println(deployLine)
				trcshDriverConfig.FeatherCtx = featherCtx
				region := ""
				if len(trcshDriverConfig.DriverConfig.CoreConfig.Regions) > 0 {
					region = trcshDriverConfig.DriverConfig.CoreConfig.Regions[0]
				}

				processPluginCmds(
					&trcKubeDeploymentConfig,
					&onceKubeInit,
					PipeOS,
					trcshDriverConfig.DriverConfig.CoreConfig.Env,
					region,
					trcshDriverConfig,
					control,
					argsOrig,
					strings.Split(deployLine, " "),
					&configCount)
			}
		}
	}
	if *dronePtr && !gTrcshConfig.IsShellRunner {
		completeOnce := false
		for {
			if atomic.LoadInt64(&featherCtx.RunState) == cap.RUNNING {
				if !completeOnce {
					closeCleanupMessaging(trcshDriverConfig)
					completeOnce = true
				}
				time.Sleep(time.Second)
			} else {
				break
			}
		}
		content = nil
		goto collaboratorReRun
	}
	// Make the arguments in the script -> os.args.
}

func closeCleanupMessaging(trcshDriverConfig *capauth.TrcshDriverConfig) {
	lastCtlChanLen := 0
	waitCtr := 0
	for {
		ctlChanLen := len(trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan)
		if ctlChanLen > 0 && waitCtr < 10 {
			if lastCtlChanLen != ctlChanLen {
				waitCtr = 0
			} else {
				waitCtr++
			}
			lastCtlChanLen = ctlChanLen
			time.Sleep(1 * time.Second)
		} else {
			if waitCtr == 10 {
				for {
					if len(trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan) > 0 {
						select {
						case <-trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan:
						default:
							break
						}
					} else {
						time.Sleep(1 * time.Second)
					}
				}
			} else {
				trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- "..."
			}
			trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- cap.CTL_COMPLETE
			atomic.StoreInt64(&trcshDriverConfig.FeatherCtx.RunState, cap.RUN_STARTED)
			break
		}
	}
}
