package trcshbase

import (
	"bytes"
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
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/trimble-oss/tierceron-hat/cap"
	captiplib "github.com/trimble-oss/tierceron-hat/captip/captiplib"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/pluginutil"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	trcshMemFs "github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/deployutil"
	kube "github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/trcshauth"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/kernelopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"gopkg.in/yaml.v2"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var gAgentConfig *capauth.AgentConfigs = nil
var gTrcshConfig *capauth.TrcShConfig
var kernelPluginHandler *hive.PluginHandler = nil

var (
	MODE_PERCH_STR string = string([]byte{cap.MODE_PERCH})
)

const (
	YOU_SHALL_NOT_PASS = "you shall not pass"
)

func CreateLogFile() (*log.Logger, error) {
	logFile := "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"deploy.log" {
		logFile = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
	}
	var f *os.File
	var logPrefix string = "[DEPLOY]"
	if kernelopts.BuildOptions.IsKernel() {
		logPrefix = "[trcshk]"
	}

	var errOpenFile error
	f, errOpenFile = os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if errOpenFile != nil {
		return nil, errOpenFile
	}
	logger := log.New(f, logPrefix, log.LstdFlags)
	return logger, nil
}

func TrcshInitConfig(driverConfigPtr *config.DriverConfig, env string, region string, pathParam string, outputMemCache bool, logger ...*log.Logger) (*capauth.TrcshDriverConfig, error) {
	if len(env) == 0 {
		env = os.Getenv("TRC_ENV")
	}
	if len(region) == 0 {
		region = os.Getenv("TRC_REGION")
	}

	regions := []string{}
	if strings.HasPrefix(env, "staging") || strings.HasPrefix(env, "prod") || strings.HasPrefix(env, "dev") {
		if strings.HasPrefix(env, "staging") || strings.HasPrefix(env, "prod") {
			prod.SetProd(true)
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
				fmt.Println("Unsupported region: " + region)
				return nil, errors.New("Unsupported region: " + region)
			}
		}
	}

	fmt.Println("trcsh env: " + env)
	fmt.Printf("trcsh regions: %s\n", strings.Join(regions, ", "))

	//Check if logger passed in - if not call create log method that does following below...
	var providedLogger *log.Logger
	var err error
	if len(logger) == 0 && driverConfigPtr == nil && driverConfigPtr.CoreConfig.Log == nil {
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

	trcshDriverConfig := &capauth.TrcshDriverConfig{
		DriverConfig: &config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				IsShell:       true,
				TokenCache:    cache.NewTokenCacheEmpty(),
				Insecure:      false,
				Env:           env,
				EnvBasis:      eUtils.GetEnvBasis(env),
				Regions:       regions,
				ExitOnFailure: true,
				Log:           providedLogger,
			},
			IsShellSubProcess: false,
			OutputMemCache:    outputMemCache,
			MemFs: &trcshMemFs.TrcshMemFs{
				BillyFs: memfs.New(),
			},
			ZeroConfig: true,
			PathParam:  pathParam, // Make available to trcplgtool
		},
	}
	return trcshDriverConfig, nil
}

// Logging of deployer controller activities..
func deployerCtlEmote(featherCtx *cap.FeatherContext, ctlFlapMode string, msg string) {
	if strings.HasSuffix(ctlFlapMode, cap.CTL_COMPLETE) {
		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
		os.Exit(0)
	}

	if len(ctlFlapMode) > 0 && ctlFlapMode[0] == cap.MODE_FLAP {
		fmt.Printf("%s\n", msg)
	}
	deployerId, _ := deployopts.BuildOptions.GetDecodedDeployerId(*featherCtx.SessionIdentifier)
	featherCtx.Log.Printf("deployer: %s ctl: %s  msg: %s\n", deployerId, ctlFlapMode, strings.Trim(msg, "\n"))
	if strings.Contains(msg, "encountered errors") {
		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
		os.Exit(0)
	}
}

// Logging of deployer activities..
func deployerEmote(featherCtx *cap.FeatherContext, ctlFlapMode []byte, msg string) {
	if len(ctlFlapMode) > 0 && ctlFlapMode[0] != cap.MODE_PERCH && msg != captiplib.MSG_PERCH_AND_GAZE {
		featherCtx.Log.Printf(msg + "\n")
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
	os.Exit(-1)
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

// EnableDeploy - initializes and starts running deployer for provided deployment and environment.
func EnableDeployer(driverConfigPtr *config.DriverConfig, env string, region string, token string, trcPath string, secretId *string, approleId *string, outputMemCache bool, deployment string, dronePtr *bool, projectService ...*string) {
	trcshDriverConfig, err := TrcshInitConfig(driverConfigPtr, env, region, "", outputMemCache)
	if err != nil {
		fmt.Printf("Initialization setup error: %s\n", err.Error())
	}
	if len(deployment) > 0 {
		// Set the name of the plugin to deploy in "trcplugin"
		// Used later by codedeploy
		trcshDriverConfig.DriverConfig.DeploymentConfig = map[string]interface{}{"trcplugin": deployment}
		trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan = make(chan string, 5)
		fmt.Printf("Starting deployer: %s\n", deployment)
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Starting deployer: %s\n", deployment)
	}

	//
	// Each deployer needs it's own context.
	//
	localHostAddr := ""
	var sessionIdentifier string
	if sessionId, ok := deployopts.BuildOptions.GetEncodedDeployerId(deployment, *gAgentConfig.Env); ok {
		sessionIdentifier = sessionId
	} else {
		fmt.Printf("Unsupported deployer: %s\n", deployment)
		os.Exit(-1)
	}
	trcshDriverConfig.FeatherCtx = captiplib.FeatherCtlInit(interruptChan,
		&localHostAddr,
		gAgentConfig.EncryptPass,
		gAgentConfig.EncryptSalt,
		gAgentConfig.HostAddr,
		gAgentConfig.HandshakeCode,
		&sessionIdentifier, /*Session identifier */
		&env,
		deployerAcceptRemoteNoTimeout,
		deployerInterrupted)
	trcshDriverConfig.FeatherCtx.Log = trcshDriverConfig.DriverConfig.CoreConfig.Log
	// featherCtx initialization is delayed for the self contained deployments (kubernetes, etc...)
	atomic.StoreInt64(&trcshDriverConfig.FeatherCtx.RunState, cap.RUN_STARTED)

	go captiplib.FeatherCtlEmitter(trcshDriverConfig.FeatherCtx, trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan, deployerEmote, nil)
	var projServ = ""
	if len(projectService) > 0 && kernelopts.BuildOptions.IsKernel() {
		projServ = *projectService[0]
	}

	go ProcessDeploy(trcshDriverConfig.FeatherCtx, trcshDriverConfig, deployment, trcPath, projServ, secretId, approleId, false, dronePtr)
}

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.

func CommonMain(envPtr *string, addrPtr *string, envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	driverConfigPtr *config.DriverConfig) error {

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
	}

	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	eUtils.InitHeadless(true)
	var regionPtr, trcPathPtr, projectServicePtr *string
	var dronePtr *bool
	// Initiate signal handling.
	var ic chan os.Signal = make(chan os.Signal, 5)

	regionPtr = flagset.String("region", "", "Region to be processed")  //If this is blank -> use context otherwise override context.
	trcPathPtr = flagset.String("c", "", "Optional script to execute.") //If this is blank -> use context otherwise override context.

	if kernelopts.BuildOptions.IsKernel() {
		dronePtr = new(bool)
		*dronePtr = true
	} else {
		dronePtr = flagset.Bool("drone", false, "Run as drone.")
	}
	if !eUtils.IsWindows() {
		if os.Geteuid() == 0 {
			fmt.Println("Trcsh cannot be run as root.")
			os.Exit(-1)
		} else {
			util.CheckNotSudo()
		}

		if len(os.Args) > 1 {
			if strings.Contains(os.Args[1], "trc") && !strings.Contains(os.Args[1], "-c") {
				// Running as shell.
				os.Args[1] = "-c=" + os.Args[1]
			}
		}
		projectServicePtr = flagset.String("projectService", "", "Service namespace to pull templates from if not present in LFS")
		signal.Notify(ic, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT)
	} else {
		dronePtr = new(bool)
		*dronePtr = true

		signal.Notify(ic, os.Interrupt)
	}
	go func() {
		x := <-ic
		interruptChan <- x
	}()

	flagset.Parse(argLines[1:])

	if !*dronePtr {
		if len(*appRoleIDPtr) == 0 {
			*appRoleIDPtr = os.Getenv("DEPLOY_ROLE")
		}

		if len(*secretIDPtr) == 0 {
			*secretIDPtr = os.Getenv("DEPLOY_SECRET")
		}

		var pathParam = os.Getenv("PATH_PARAM")

		memprotectopts.MemProtect(nil, secretIDPtr)
		memprotectopts.MemProtect(nil, appRoleIDPtr)

		trcshDriverConfig, err := TrcshInitConfig(driverConfigPtr, *envPtr, *regionPtr, pathParam, true)
		if err != nil {
			fmt.Printf("trcsh config setup failure: %s\n", err.Error())
			os.Exit(124)
		}

		//Open deploy script and parse it.
		ProcessDeploy(nil, trcshDriverConfig, "", *trcPathPtr, *projectServicePtr, secretIDPtr, appRoleIDPtr, true, dronePtr)
	} else {
		if driverConfigPtr != nil && driverConfigPtr.CoreConfig.Log == nil {
			logger, err := CreateLogFile()
			if err != nil {
				fmt.Printf("Error initializing log file: %s\n", err)
				os.Exit(-1)
			}
			driverConfigPtr.CoreConfig.Log = logger
		}

		if kernelopts.BuildOptions.IsKernel() {
			go deployutil.KernelShutdownWatcher(driverConfigPtr.CoreConfig.Log)
		}
		var agentEnv string
		var addressPtr *string
		var deploymentsShard string
		fromWinCred := false
		useRole := true

		if kernelopts.BuildOptions.IsKernel() {
			// load via new properties and get config values
			data, err := os.ReadFile("config.yml")
			if err != nil {
				driverConfigPtr.CoreConfig.Log.Println("Error reading YAML file:", err)
				os.Exit(-1) //do we want to exit???
			}
			// Create an empty map for the YAML data
			var config map[string]interface{}

			// Unmarshal the YAML data into the map
			err = yaml.Unmarshal(data, &config)
			if err != nil {
				driverConfigPtr.CoreConfig.Log.Println("Error unmarshaling YAML:", err)
				os.Exit(-1)
			}
			if role, ok := config["agent_role"].(string); ok {
				app_sec := strings.Split(role, ":")
				if len(app_sec) != 2 {
					fmt.Println("invalid agent role used for drone trcsh agent")
					driverConfigPtr.CoreConfig.Log.Println("invalid agent role used for drone trcsh agent")
					os.Exit(124)
				}
				appRoleIDPtr = &app_sec[0]
				secretIDPtr = &app_sec[1]
			} else {
				useRole = false
				driverConfigPtr.CoreConfig.Log.Println("Error reading config value")
			}
			if addr, ok := config["vault_addr"].(string); ok {
				addressPtr = &addr
			} else {
				driverConfigPtr.CoreConfig.Log.Println("Error reading config value")
			}
			if env, ok := config["agent_env"].(string); ok {
				agentEnv = env
			} else {
				driverConfigPtr.CoreConfig.Log.Println("Error reading config value")
			}
			if deployments, ok := config["deployments"].(string); ok {
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
						fmt.Println("Error loading authentication from Credential Manager")
						driverConfigPtr.CoreConfig.Log.Println("Error loading authentication from Credential Manager")
						useRole = false
					} else {
						agentRole := string(role.CredentialBlob)
						fromWinCred = true
						app_sec := strings.Split(agentRole, ":")
						if len(app_sec) != 2 {
							fmt.Println("invalid agent role used from wincred for drone trcsh agent")
							driverConfigPtr.CoreConfig.Log.Println("invalid agent role used from wincred for drone trcsh agent")
							os.Exit(124)
						}
						appRoleIDPtr = &app_sec[0]
						secretIDPtr = &app_sec[1]
					}
				} else {
					app_sec := strings.Split(agentRole, ":")
					if len(app_sec) != 2 {
						fmt.Println("invalid agent role used from wincred for drone trcsh agent")
						driverConfigPtr.CoreConfig.Log.Println("invalid agent role used from wincred for drone trcsh agent")
						os.Exit(124)
					}
					appRoleIDPtr = &app_sec[0]
					secretIDPtr = &app_sec[1]
				}

			} else {
				agentRole := os.Getenv("AGENT_ROLE")
				if agentRole == "" {
					fmt.Println("Error loading authentication from env")
					driverConfigPtr.CoreConfig.Log.Println("Error loading authentication from env")
					useRole = false
				} else {
					app_sec := strings.Split(agentRole, ":")
					if len(app_sec) != 2 {
						fmt.Println("invalid agent role used from wincred for drone trcsh agent")
						driverConfigPtr.CoreConfig.Log.Println("invalid agent role used from wincred for drone trcsh agent")
						os.Exit(124)
					}
					appRoleIDPtr = &app_sec[0]
					secretIDPtr = &app_sec[1]
				}
			}
			agentEnv = os.Getenv("AGENT_ENV")
			addressPtr = new(string)
			*addressPtr = os.Getenv("VAULT_ADDR")

			//Replace dev-1 with DEPLOYMENTS-1
			deploymentsKey := "DEPLOYMENTS"
			subDeploymentIndex := strings.Index(*envPtr, "-")
			if subDeploymentIndex != -1 {
				deploymentsKey += (*envPtr)[subDeploymentIndex:]
			}
			deploymentsShard = os.Getenv(deploymentsKey)

			if len(deploymentsShard) == 0 {
				deploymentsShard = os.Getenv(strings.Replace(deploymentsKey, "-", "_", 1))
				if len(deploymentsShard) == 0 {
					fmt.Printf("drone trcsh requires a %s.\n", deploymentsKey)
					driverConfigPtr.CoreConfig.Log.Printf("drone trcsh requires a %s.\n", deploymentsKey)
					os.Exit(-1)
				}
			}
		}

		if !useRole && !eUtils.IsWindows() {
			fmt.Println("drone trcsh requires AGENT_ROLE.")
			driverConfigPtr.CoreConfig.Log.Println("drone trcsh requires AGENT_ROLE.")
			os.Exit(-1)
		}

		if len(agentEnv) == 0 {
			fmt.Println("drone trcsh requires AGENT_ENV.")
			driverConfigPtr.CoreConfig.Log.Println("drone trcsh requires AGENT_ENV.")
			os.Exit(-1)
		}

		if len(*envPtr) > 0 {
			agentEnv = *envPtr
		}

		if eUtils.RefLength(addressPtr) == 0 {
			fmt.Println("drone trcsh requires VAULT_ADDR address.")
			driverConfigPtr.CoreConfig.Log.Println("drone trcsh requires VAULT_ADDR address.")
			os.Exit(-1)
		}

		if err := capauth.ValidateVhost(*addressPtr, "https://", false, driverConfigPtr.CoreConfig.Log); err != nil {
			fmt.Printf("drone trcsh requires supported VAULT_ADDR address: %s\n", err.Error())
			driverConfigPtr.CoreConfig.Log.Printf("drone trcsh requires supported VAULT_ADDR address: %s\n", err.Error())
			os.Exit(124)
		}

		if kernelopts.BuildOptions.IsKernel() {
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
			if matches, _ := regexp.MatchString("trcshk\\-\\d+$", hostname); matches {
				driverConfigPtr.CoreConfig.Log.Println("Stateful set enabled")

				// spectrum-aggregator-snapshot-<pool>
				hostParts := strings.Split(hostname, "-")
				var err error
				id, err = strconv.Atoi(hostParts[1])
				if err != nil {
					id = 0
				}
				driverConfigPtr.CoreConfig.Log.Printf("Starting Stateful trcshk with set entry id: %d\n", id)
			} else {
				driverConfigPtr.CoreConfig.Log.Printf("Unable to match: %s\n", hostname)
			}
			if id > 0 {
				agentEnv = fmt.Sprintf("%s-%d", agentEnv, id)
			}
			driverConfigPtr.CoreConfig.Log.Printf("Identified as: %s\n", agentEnv)
		}

		memprotectopts.MemProtect(nil, addressPtr)
		shutdown := make(chan bool)

		fmt.Printf("drone trcsh beginning new agent configuration sequence.\n")
		driverConfigPtr.CoreConfig.Log.Printf("drone trcsh beginning new agent configuration sequence.\n")
		// Preload agent synchronization configs...
		trcshDriverConfig, err := TrcshInitConfig(driverConfigPtr, agentEnv, *regionPtr, "", true, driverConfigPtr.CoreConfig.Log)
		if err != nil {
			fmt.Printf("drone trcsh agent bootstrap init config failure: %s\n", err.Error())
			driverConfigPtr.CoreConfig.Log.Printf("drone trcsh agent bootstrap init config failure: %s\n", err.Error())
			os.Exit(124)
		}

		if useRole {
			trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr = addressPtr
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth..")

			authTokenEnv := agentEnv
			appRoleConfig := "hivekernel"
			authTokenName := fmt.Sprintf("trcsh_agent_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
			trcshEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

			tokenPtr := new(string)
			autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, secretIDPtr, appRoleIDPtr, &authTokenName, tokenPtr, &authTokenEnv, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &trcshEnvBasis, &appRoleConfig, false)
			if autoErr != nil || eUtils.RefLength(trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken(authTokenName)) == 0 {
				fmt.Println("Unable to auth.")
				if autoErr != nil {
					fmt.Println(autoErr)
				}
				os.Exit(-1)
			}
		}

		var errAgentLoad error
		gAgentConfig, gTrcshConfig, errAgentLoad = capauth.NewAgentConfig(addressPtr,
			trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("trcsh_agent_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)),
			agentEnv, deployCtlAcceptRemoteNoTimeout, nil, true, driverConfigPtr.CoreConfig.Log, dronePtr)
		if errAgentLoad != nil {
			// check os.env for another token
			fmt.Printf("drone trcsh agent bootstrap agent config failure: %s\n", errAgentLoad.Error())
			driverConfigPtr.CoreConfig.Log.Printf("drone trcsh agent bootstrap agent config failure: %s\n", errAgentLoad.Error())
			os.Exit(124)
		}

		fmt.Println("Drone trcsh agent bootstrap successful.")
		driverConfigPtr.CoreConfig.Log.Println("Drone trcsh agent bootstrap successful.")

		if eUtils.IsWindows() {
			if !fromWinCred {
				// migrate token to wincred
				var cred *wincred.GenericCredential
				if useRole {
					cred = wincred.NewGenericCredential("AGENT_ROLE")
					cred.CredentialBlob = []byte(*appRoleIDPtr + ":" + *secretIDPtr)
					err := cred.Write()
					if err != nil {
						fmt.Printf("Error migrating updated role: %s\n", err)
						driverConfigPtr.CoreConfig.Log.Printf("Error migrating updated role: %s\n", err)
					} else {
						//delete os.env token
						if os.Getenv("AGENT_TOKEN") != "" {
							command := exec.Command("cmd", "/C", "setx", "/M", "AGENT_TOKEN", "UNSET")
							_, err := command.CombinedOutput()
							if err != nil {
								fmt.Println(err)
								driverConfigPtr.CoreConfig.Log.Println(err)
							}
						}
						if os.Getenv("AGENT_ROLE") != "" {
							command := exec.Command("cmd", "/C", "setx", "/M", "AGENT_ROLE", "UNSET")
							_, err := command.CombinedOutput()
							if err != nil {
								fmt.Println(err)
								driverConfigPtr.CoreConfig.Log.Println(err)
							}
						}
					}
				} else {
					fmt.Printf("Error migrating updated role or token: %s\n", err)
					driverConfigPtr.CoreConfig.Log.Printf("Error migrating updated role or token: %s\n", err)
				}
			}
		}
		if useRole {
			//
			// Zero after use to prevent downstream conflicts or reliance.
			//
			if appRoleIDPtr != nil {
				*appRoleIDPtr = ""
			}
			if secretIDPtr != nil {
				*secretIDPtr = ""
			}
		}

		fmt.Printf("drone trcsh beginning initialization sequence.\n")
		driverConfigPtr.CoreConfig.Log.Printf("drone trcsh beginning initialization sequence.\n")
		// Initialize deployers.

		// Validate drone sha path
		pluginConfig := make(map[string]interface{})
		pluginConfig["vaddress"] = *addressPtr

		currentTokenName := fmt.Sprintf("trcsh_agent_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		pluginConfig["tokenptr"] = trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken(currentTokenName)
		pluginConfig["env"] = trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

		if eUtils.IsWindows() {
			pluginConfig["plugin"] = "trcsh.exe"
		} else if kernelopts.BuildOptions.IsKernel() {
			pluginConfig["plugin"] = "trcshk"
		} else {
			pluginConfig["plugin"] = "trcsh"
		}

		_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, currentTokenName, driverConfigPtr.CoreConfig.Log)
		if err != nil {
			fmt.Printf("Problem initializing mod: %s\n", err)
			driverConfigPtr.CoreConfig.Log.Printf("Problem initializing mod: %s\n", err)
		}
		if vault != nil {
			defer vault.Close()
		}

		isValid, err := trcshauth.ValidateTrcshPathSha(mod, pluginConfig, driverConfigPtr.CoreConfig.Log)
		if err != nil || !isValid {
			fmt.Printf("Error obtaining authorization components: %s\n", err)
			os.Exit(124)
		}

		if kernelopts.BuildOptions.IsKernel() && kernelPluginHandler == nil {
			kernelPluginHandler = hive.InitKernel()
		}

		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Completed bootstrapping and continuing to initialize services.")
		trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfigPtr = gTrcshConfig.ConfigRolePtr
		trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr = gTrcshConfig.VaultAddressPtr

		serviceDeployments, err := deployutil.GetDeployers(trcshDriverConfig, dronePtr)
		if err != nil {
			fmt.Printf("drone trcsh agent bootstrap get deployers failure: %s\n", err.Error())
			os.Exit(124)
		}
		deploymentShards := strings.Split(deploymentsShard, ",")
		deployments := []string{}

		// This is a tad more complex but will scale more nicely.
		deploymentShardsSet := map[string]struct{}{}
		for _, str := range deploymentShards {
			deploymentShardsSet[str] = struct{}{}
		}

		for _, serviceDeployment := range serviceDeployments {
			if _, ok := deploymentShardsSet[serviceDeployment]; ok {
				deployments = append(deployments, serviceDeployment)
				if kernelopts.BuildOptions.IsKernel() && kernelPluginHandler != nil {
					kernelPluginHandler.AddKernelPlugin(serviceDeployment, trcshDriverConfig.DriverConfig)
				}
			}
		}
		deploymentsCDL := strings.Join(deployments, ",")
		gAgentConfig.Deployments = &deploymentsCDL

		deployopts.BuildOptions.InitSupportedDeployers(deployments)

		if len(deployments) == 0 {
			fmt.Println("No valid deployments for trcshell, entering hibernate mode.")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("No valid deployments for trcshell, entering hibernate mode.")
			hibernate := make(chan bool)
			hibernate <- true
		}

		if kernelopts.BuildOptions.IsKernel() && kernelPluginHandler != nil {
			go func(kpH *hive.PluginHandler, dc *config.DriverConfig) {
				kpH.Handle_Chat(dc)
			}(kernelPluginHandler, trcshDriverConfig.DriverConfig)
		}

		for _, deployment := range deployments {
			EnableDeployer(driverConfigPtr, *gAgentConfig.Env, *regionPtr, deployment, *trcPathPtr, secretIDPtr, appRoleIDPtr, false, deployment, dronePtr, projectServicePtr)
		}

		<-shutdown
	}
	return nil
}

var interruptChan chan os.Signal = make(chan os.Signal, 5)
var twoHundredMilliInterruptTicker *time.Ticker = time.NewTicker(200 * time.Millisecond)
var secondInterruptTicker *time.Ticker = time.NewTicker(time.Second)
var multiSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 3)
var fiveSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 5)
var fifteenSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 5)
var thirtySecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 5)

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
		os.Exit(128)
	}
	return result, resultError
}

func acceptInterruptNoTimeoutFun(featherCtx *cap.FeatherContext, tickerContinue *time.Ticker) (bool, error) {
	result := false
	var resultError error = nil
	select {
	case <-tickerContinue.C:
		// don't break... continue...
		result = false
		resultError = nil
	}
	if len(featherCtx.InterruptChan) > 0 {
		cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
		os.Exit(128)
	}
	return result, resultError
}

func interruptFun(featherCtx *cap.FeatherContext, tickerInterrupt *time.Ticker) {
	select {
	case <-tickerInterrupt.C:
		if len(featherCtx.InterruptChan) > 0 {
			cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
			os.Exit(128)
		}
	}
}

// acceptRemote - hook for instrumenting
func acceptRemote(featherCtx *cap.FeatherContext, mode int, remote string) (bool, error) {
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

	if sessionIdentifier, ok := deployopts.BuildOptions.GetEncodedDeployerId(agentName, *featherCtx.Env); ok {
		featherCtx.SessionIdentifier = &sessionIdentifier
		featherCtx.Log.Printf("Starting deploy ctl session: %s\n", sessionIdentifier)
		captiplib.FeatherCtl(featherCtx, deployerCtlEmote)
	} else {
		fmt.Printf("Unsupported agent: %s\n", agentName)
		os.Exit(123) // Missing config.
	}

	return nil
}

func roleBasedRunner(
	region string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	control string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) error {
	*configCount -= 1
	approleconfigPtr := new(string)
	*approleconfigPtr = "config.yml"
	trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfigPtr = approleconfigPtr
	trcshDriverConfig.DriverConfig.FileFilter = nil
	trcshDriverConfig.DriverConfig.CoreConfig.WantCerts = false
	trcshDriverConfig.DriverConfig.IsShellSubProcess = true
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner init: %s\n", control)

	if eUtils.RefEquals(trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, "") {
		trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr = gTrcshConfig.VaultAddressPtr
	}
	if trcDeployRoot, ok := trcshDriverConfig.DriverConfig.DeploymentConfig["trcdeployroot"]; ok {
		trcshDriverConfig.DriverConfig.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
		trcshDriverConfig.DriverConfig.EndDir = trcDeployRoot.(string)
	} else {
		trcshDriverConfig.DriverConfig.StartDir = []string{"trc_templates"}
		trcshDriverConfig.DriverConfig.EndDir = "."
	}
	configRoleSlice := strings.Split(*gTrcshConfig.ConfigRolePtr, ":")
	tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	envDefaultPtr := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	var err error
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner started: %s\n", control)

	switch control {
	case "trcplgtool":
		envDefaultPtr = trcshDriverConfig.DriverConfig.CoreConfig.Env
		tokenName = "config_token_pluginany"
		if kernelopts.BuildOptions.IsKernel() {
			err = trcplgtoolbase.CommonMain(&envDefaultPtr, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &gTrcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, deployArgLines, trcshDriverConfig, kernelPluginHandler)
		} else {
			err = trcplgtoolbase.CommonMain(&envDefaultPtr, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &gTrcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, deployArgLines, trcshDriverConfig)
		}
	case "trcconfig":
		if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" || trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "staging" || trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "prod" ||
			trcshDriverConfig.DriverConfig.CoreConfig.Env == "itdev" || trcshDriverConfig.DriverConfig.CoreConfig.Env == "staging" || trcshDriverConfig.DriverConfig.CoreConfig.Env == "prod" {
			trcshDriverConfig.DriverConfig.OutputMemCache = false
			// itdev, staging, and prod always key off TRC_ENV stored in trcshDriverConfig.DriverConfig.CoreConfig.Env.
			envDefaultPtr = trcshDriverConfig.DriverConfig.CoreConfig.Env
			tokenName = "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.Env
		}
		err = trcconfigbase.CommonMain(&envDefaultPtr, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &gTrcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, deployArgLines, trcshDriverConfig.DriverConfig)
	case "trcsub":
		trcshDriverConfig.DriverConfig.EndDir = trcshDriverConfig.DriverConfig.EndDir + "/trc_templates"
		err = trcsubbase.CommonMain(&envDefaultPtr, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &gTrcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], nil, deployArgLines, trcshDriverConfig.DriverConfig)
	}
	ResetModifier(trcshDriverConfig.DriverConfig.CoreConfig, tokenName) //Resetting modifier cache to avoid token conflicts.
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner complete: %s\n", control)

	return err
}

func processPluginCmds(trcKubeDeploymentConfig **kube.TrcKubeConfig,
	onceKubeInit *sync.Once,
	PipeOS billy.File,
	env string,
	region string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	control string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) {

	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Processing control: %s\n", control)

	switch control {
	case "trccertinit":
		if prod.IsProd() {
			fmt.Printf("trccertinit unsupported in production\n")
			os.Exit(125) // Running functionality not supported in prod.
		}
		tokenName := fmt.Sprintf("vault_pub_token_%s", env)
		ResetModifier(trcshDriverConfig.DriverConfig.CoreConfig, tokenName) //Resetting modifier cache to avoid token conflicts.
		approleconfigPtr := new(string)
		*approleconfigPtr = "configpub.yml"
		trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfigPtr = approleconfigPtr
		trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis = env
		trcshDriverConfig.DriverConfig.IsShellSubProcess = true
		trcshDriverConfig.DriverConfig.CoreConfig.WantCerts = true
		if len(*gTrcshConfig.PubRolePtr) == 0 {
			fmt.Printf("Missing required certification auth components\n")
			os.Exit(125)
		}

		pubRoleSlice := strings.Split(*gTrcshConfig.PubRolePtr, ":")
		pubEnv := env

		trcinitbase.CommonMain(&pubEnv, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &gTrcshConfig.EnvContext, &pubRoleSlice[1], &pubRoleSlice[0], &tokenName, &trcshDriverConfig.DriverConfig.CoreConfig.WantCerts, nil, deployArgLines, trcshDriverConfig.DriverConfig)
		ResetModifier(trcshDriverConfig.DriverConfig.CoreConfig, tokenName) //Resetting modifier cache to avoid token conflicts.
	case "trcpub":
		tokenName := fmt.Sprintf("vault_pub_token_%s", env)
		ResetModifier(trcshDriverConfig.DriverConfig.CoreConfig, tokenName) //Resetting modifier cache to avoid token conflicts.
		approleconfigPtr := new(string)
		*approleconfigPtr = "configpub.yml"
		trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfigPtr = approleconfigPtr
		trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis = env
		trcshDriverConfig.DriverConfig.IsShellSubProcess = true
		if len(*gTrcshConfig.PubRolePtr) == 0 {
			fmt.Printf("Missing required pub auth components\n")
			os.Exit(125)
		}
		pubRoleSlice := strings.Split(*gTrcshConfig.PubRolePtr, ":")
		pubEnv := env

		trcpubbase.CommonMain(&pubEnv, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &gTrcshConfig.EnvContext, &pubRoleSlice[1], &pubRoleSlice[0], &tokenName, nil, deployArgLines, trcshDriverConfig.DriverConfig)
		ResetModifier(trcshDriverConfig.DriverConfig.CoreConfig, tokenName) //Resetting modifier cache to avoid token conflicts.
	case "trcconfig":
		err := roleBasedRunner(region, trcshDriverConfig, control, argsOrig, deployArgLines, configCount)
		if err != nil {
			fmt.Println("trcconfig - unexpected failure")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println(err)
			os.Exit(1)
		}
	case "trcplgtool":
		// Utilize elevated CToken to perform certifications if asked.
		if prod.IsProd() {
			fmt.Printf("trcplgtool unsupported in production\n")
			os.Exit(125) // Running functionality not supported in prod.
		}
		trcshDriverConfig.FeatherCtlCb = featherCtlCb
		if gAgentConfig == nil {

			var errAgentLoad error
			if gTrcshConfig == nil || !gTrcshConfig.IsValid(gAgentConfig) || eUtils.RefLength(gTrcshConfig.TokenCache.GetToken("config_token_pluginany")) == 0 {
				// Chewbacca: Consider removing as this should have already
				// been done earlier in the process.
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Unexpected invalid trcshConfig.  Attempting recovery.")
				retries := 0
				for {
					if gTrcshConfig == nil || !gTrcshConfig.IsValid(gAgentConfig) {
						var err error
						// Loop until we have something usable...
						gTrcshConfig, err = trcshauth.TrcshAuth(nil, gAgentConfig, trcshDriverConfig)
						if err != nil {
							trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf(".")
							time.Sleep(time.Second)
							retries = retries + 1
							if retries >= 7 {
								fmt.Printf("Unexpected nil trcshConfig.  Cannot continue.\n")
								os.Exit(124) // Setup problem.
							}
							continue
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
			gAgentConfig, _, errAgentLoad = capauth.NewAgentConfig(gTrcshConfig.VaultAddressPtr,
				gTrcshConfig.TokenCache.GetToken("config_token_pluginany"),
				env,
				deployCtlAcceptRemote,
				deployCtlInterrupted,
				false,
				trcshDriverConfig.DriverConfig.CoreConfig.Log)
			if errAgentLoad != nil {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Permissions failure.  Incorrect deployment\n")
				fmt.Printf("Permissions failure.  Incorrect deployment\n")
				os.Exit(126) // possible token permissions issue
			}
			if gAgentConfig.FeatherContext == nil {
				fmt.Printf("Warning!  Permissions failure.  Incorrect feathering\n")
			}
			gAgentConfig.InterruptHandlerFunc = deployCtlInterrupted
		}
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

		trcshDriverConfig.DriverConfig.CoreConfig.TokenCache = gTrcshConfig.TokenCache
		err := roleBasedRunner(region, trcshDriverConfig, control, argsOrig, deployArgLines, configCount)
		if err != nil {
			fmt.Println("trcplgtool - unexpected failure")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println(err)
			os.Exit(1)
		}

	case "kubectl":
		onceKubeInit.Do(func() {
			var kubeInitErr error
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Setting up kube config")
			*trcKubeDeploymentConfig, kubeInitErr = kube.InitTrcKubeConfig(gTrcshConfig, trcshDriverConfig.DriverConfig.CoreConfig)
			if kubeInitErr != nil {
				fmt.Println(kubeInitErr)
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
			fmt.Println("Kubernetes connection stalled or timed out.  Possible kubernetes ip change")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Timed out waiting for KubeCtl.")
			os.Exit(-1)
		case kubeErr := <-kubectlErrChan:
			if kubeErr != nil {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Println(kubeErr)
				os.Exit(-1)
			}
		}
	}
}

func processDroneCmds(trcKubeDeploymentConfig *kube.TrcKubeConfig,
	onceKubeInit *sync.Once,
	PipeOS billy.File,
	region string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	control string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) error {

	err := roleBasedRunner(region, trcshDriverConfig, control, argsOrig, deployArgLines, configCount)
	return err
}

// ProcessDeploy
//
// Parameters:
//
//   - env: Current environment context
//   - region: a region
//   - token: An environment token
//   - deployment: name of deployment
//   - trcPath: Path to the current deployment script...
//   - secretId: trcsh secret.
//   - approleId: trcsh app role.
//
// Returns:
//
//	Nothing.
func ProcessDeploy(featherCtx *cap.FeatherContext,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	deployment string,
	trcPath string,
	projectServicePtr string,
	secretId *string,
	approleId *string,
	outputMemCache bool,
	dronePtr *bool) {
	// Verify Billy implementation
	configMemFs := trcshDriverConfig.DriverConfig.MemFs.(*trcshMemFs.TrcshMemFs)

	pwd, _ := os.Getwd()
	var content []byte

	if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" {
		trcshDriverConfig.DriverConfig.OutputMemCache = false
	}
	fmt.Println("Logging initialized.")
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Logging initialized for env:%s\n", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)

	var err error
	vaultAddress, err := trcshauth.TrcshVAddress(featherCtx, gAgentConfig, trcshDriverConfig)
	// Chewbacca: scrub before checkin
	// This data is generated by TrcshAuth
	// var vaultAddress *string
	// vAddress := os.Getenv("VAULT_ADDR")
	// vaultAddress = &vAddress
	// err = nil
	//	Chewbacca: end scrub
	if err != nil || len(*vaultAddress) == 0 {
		fmt.Println("Auth phase 0 failure")
		if err != nil {
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Error: %s\n", err.Error())
		}
		os.Exit(-1)
	}
	trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr = vaultAddress
	// Chewbacca: scrub before checkin
	// This data is generated by TrcshAuth
	// configRole := os.Getenv("CONFIG_ROLE")
	// pubRole := os.Getenv("PUB_ROLE")
	// pluginAny := os.Getenv("PLUGIN_ANY")
	// fileBytes, _ := os.ReadFile("")
	// kc := base64.StdEncoding.EncodeToString(fileBytes)
	// gTrcshConfig = &capauth.TrcShConfig{Env: "dev",
	// 	EnvContext:    "dev",
	// 	TokenCache:    trcshDriverConfig.DriverConfig.CoreConfig.TokenCache,
	// 	ConfigRolePtr: &configRole,
	// 	PubRolePtr:    &pubRole,
	// 	KubeConfigPtr: &kc,
	// }
	// vAddr := os.Getenv("VAULT_ADDR")
	// trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr = &vAddr
	// gTrcshConfig.VaultAddressPtr = trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr
	// trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.AddToken("config_token_pluginany", &pluginAny)
	//	Chewbacca: end scrub
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth..")

	trcshEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	deployTokenPtr := new(string)
	authTokenEnv := "azuredeploy"
	appRoleConfig := "deployauth"
	if gAgentConfig != nil && gAgentConfig.AgentToken != nil {
		deployTokenPtr = gAgentConfig.AgentToken
		appRoleConfig = "none"
	}
	authTokenName := "vault_token_azuredeploy"
	autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, secretId, approleId, &authTokenName, deployTokenPtr, &authTokenEnv, trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr, &trcshEnvBasis, &appRoleConfig, false)
	if autoErr != nil || eUtils.RefLength(trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken("vault_token_azuredeploy")) == 0 {
		fmt.Println("Unable to auth.")
		if autoErr != nil {
			fmt.Println(autoErr)
		}
		os.Exit(-1)
	}
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Bootstrap..")
	for {
		if gTrcshConfig == nil || !gTrcshConfig.IsValid(gAgentConfig) {
			// Loop until we have something usable...
			gTrcshConfig, err = trcshauth.TrcshAuth(featherCtx, gAgentConfig, trcshDriverConfig)
			if err != nil {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf(".")
				time.Sleep(time.Second)
				continue
			}
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth re-loaded %s\n", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		} else {
			break
		}
	}
	// Chewbacca: Begin dbg comment
	mergedVaultAddressPtr := trcshDriverConfig.DriverConfig.CoreConfig.VaultAddressPtr
	mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

	if eUtils.RefLength(mergedVaultAddressPtr) == 0 {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil && gTrcshConfig.VaultAddressPtr != nil {
			mergedVaultAddressPtr = gTrcshConfig.VaultAddressPtr
		}
	}

	if len(mergedEnvBasis) == 0 {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil {
			mergedEnvBasis = gTrcshConfig.EnvContext
		}
	}

	// End dbg comment
	if trcshDriverConfig.DriverConfig.CoreConfig.IsShell {
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Session Authorized")
	} else {
		fmt.Println("Session Authorized")
	}

	if kernelopts.BuildOptions.IsKernel() || ((len(os.Args) > 1) && len(trcPath) > 0) && !strings.Contains(pwd, "TrcDeploy") {
		// Generate trc code...
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Preload setup")
		configRoleSlice := strings.Split(*gTrcshConfig.ConfigRolePtr, ":")
		tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

		if kernelopts.BuildOptions.IsKernel() {
			pluginMap := map[string]interface{}{"pluginName": deployment}

			tokenPtr := new(string)
			autoErr := eUtils.AutoAuth(trcshDriverConfig.DriverConfig, &configRoleSlice[1], &configRoleSlice[0], &tokenName, tokenPtr, &mergedEnvBasis, mergedVaultAddressPtr, &mergedEnvBasis, trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfigPtr, false)
			if autoErr != nil {
				fmt.Printf("Kernel Missing auth components: %s.\n", deployment)
				return
			}

			mod, err := helperkv.NewModifier(trcshDriverConfig.DriverConfig.CoreConfig.Insecure, trcshDriverConfig.DriverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)), mergedVaultAddressPtr, mergedEnvBasis, nil, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
			if mod != nil {
				defer mod.Release()
			}
			if err != nil {
				fmt.Printf("Kernel Missing mod components: %s.\n", deployment)
				return
			}
			mod.Env = trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

			certifyMap, err := pluginutil.GetPluginCertifyMap(mod, pluginMap)
			if err != nil {
				fmt.Printf("Kernel Missing plugin certification: %s.\n", deployment)
				return
			}
			if pjService, ok := certifyMap["trcprojectservice"]; ok {
				projectServicePtr = pjService.(string)
			} else {
				fmt.Printf("Kernel Missing plugin component project service: %s.\n", deployment)
				return
			}

			if trcBootstrap, ok := certifyMap["trcbootstrap"]; ok && strings.Contains(trcBootstrap.(string), "/deploy/") {
				trcPath = trcBootstrap.(string)
			} else {
				fmt.Println("Kernel Missing plugin component bootstrap.")
				return
			}
		}

		trcPathParts := strings.Split(trcPath, "/")
		trcshDriverConfig.DriverConfig.FileFilter = []string{trcPathParts[len(trcPathParts)-1]}

		if len(configRoleSlice) != 2 {
			fmt.Println("Preload failed.  Couldn't load required resource.")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Couldn't config auth required resource.\n")
			os.Exit(124)
		}
		if projectServicePtr != "" {
			fmt.Println("Trcsh - Attempting to fetch templates from provided projectServicePtr: " + projectServicePtr)
			if !strings.Contains(trcPath, "/deploy/") {
				fmt.Println("Trcsh - Failed to fetch template using projectServicePtr.  Path is missing /deploy/")
				return
			}
			deployTrcPath := trcPath[strings.LastIndex(trcPath, "/deploy/"):]
			templatePathsPtr := projectServicePtr + strings.TrimSuffix(deployTrcPath, ".trc") // get rid of trailing .trc
			trcshDriverConfig.DriverConfig.EndDir = "./trc_templates"

			err := trcsubbase.CommonMain(&trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis, mergedVaultAddressPtr,
				&mergedEnvBasis, &configRoleSlice[1], &configRoleSlice[0], nil, []string{"trcsh", "-templatePaths=" + templatePathsPtr}, trcshDriverConfig.DriverConfig)
			if err != nil {
				fmt.Println("Trcsh - Failed to fetch template using projectServicePtr. " + err.Error())
				return
			}
			trcshDriverConfig.DriverConfig.ServicesWanted = strings.Split(projectServicePtr, ",")
		}

		trcshDriverConfig.DriverConfig.OutputMemCache = true
		trcshDriverConfig.DriverConfig.StartDir = []string{"trc_templates"}
		trcshDriverConfig.DriverConfig.EndDir = "."
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Preloading path %s env %s\n", trcPath, trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)
		region := ""
		if len(trcshDriverConfig.DriverConfig.CoreConfig.Regions) > 0 {
			region = trcshDriverConfig.DriverConfig.CoreConfig.Regions[0]
		}

		envConfig := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
		if strings.Contains(trcshDriverConfig.DriverConfig.CoreConfig.Env, "-") {
			envConfig = trcshDriverConfig.DriverConfig.CoreConfig.Env
		}

		configErr := trcconfigbase.CommonMain(&envConfig, mergedVaultAddressPtr, &mergedEnvBasis, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, []string{"trcsh"}, trcshDriverConfig.DriverConfig)
		if configErr != nil {
			fmt.Println("Preload failed.  Couldn't find required resource.")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Preload Error %s\n", configErr.Error())
			os.Exit(123)
		}
		ResetModifier(trcshDriverConfig.DriverConfig.CoreConfig, tokenName) //Resetting modifier cache to avoid token conflicts.

		var memFile billy.File
		var memFileErr error
		if memFile, memFileErr = configMemFs.BillyFs.Open(trcPath); memFileErr == nil {
			// Read the generated .trc code...
			buf := bytes.NewBuffer(nil)
			io.Copy(buf, memFile) // Error handling elided for brevity.
			content = buf.Bytes()
			configMemFs.BillyFs.Remove(trcPath)
		} else {
			if strings.HasPrefix(trcPath, "./") {
				trcPath = strings.TrimLeft(trcPath, "./")
			}
			if memFile, memFileErr = configMemFs.BillyFs.Open(trcPath); memFileErr == nil {
				// Read the generated .trc code...
				buf := bytes.NewBuffer(nil)
				io.Copy(buf, memFile) // Error handling elided for brevity.
				content = buf.Bytes()
				configMemFs.BillyFs.Remove(trcPath)
			} else {
				if strings.HasPrefix(trcPath, "./") {
					trcPath = strings.TrimLeft(trcPath, "./")
				}

				// TODO: Move this out into its own function
				fmt.Println("Trcsh - Error could not find " + trcPath + " for deployment instructions..")
			}
		}

		if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" || trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "staging" || trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "prod" {
			trcshDriverConfig.DriverConfig.OutputMemCache = false
		}
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Processing trcshell")
	} else {
		if !strings.Contains(pwd, "TrcDeploy") || len(trcshDriverConfig.DriverConfig.DeploymentConfig) == 0 {
			fmt.Println("Processing manual trcshell")
			if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" {
				content, err = os.ReadFile(pwd + "/deploy/buildtest.trc")
				if err != nil {
					fmt.Println("Trcsh - Error could not find /deploy/buildtest.trc for deployment instructions")
				}
			} else {
				content, err = os.ReadFile(pwd + "/deploy/deploy.trc")
				if err != nil {
					fmt.Println("Trcsh - Error could not find " + pwd + "/deploy/deploy.trc for deployment instructions")
					trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Trcsh - Error could not find %s/deploy/deploy.trc for deployment instructions", pwd)
				}
			}
		}
	}

collaboratorReRun:
	if featherCtx != nil && content == nil {
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
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Failure to load deployment: %s\n", trcshDriverConfig.DriverConfig.DeploymentConfig["trcplugin"])
			time.Sleep(time.Minute)
			content = nil
			goto collaboratorReRun
		}
	}

	deployArgLines := strings.Split(string(content), "\n")
	configCount := strings.Count(string(content), "trcconfig") //Uses this to close result channel on last run.
	argsOrig := os.Args

	var trcKubeDeploymentConfig *kube.TrcKubeConfig
	var onceKubeInit sync.Once
	var PipeOS billy.File

	for _, deployPipeline := range deployArgLines {
		deployPipeline = strings.TrimLeft(deployPipeline, " \t\r\n")
		if strings.HasPrefix(deployPipeline, "#") || deployPipeline == "" {
			continue
		}
		// Print current process line.
		fmt.Println(deployPipeline)
		deployPipeSplit := strings.Split(deployPipeline, "|")

		if PipeOS, err = configMemFs.BillyFs.Create("io/STDIO"); err != nil {
			fmt.Println("Failure to open io stream.")
			os.Exit(-1)
		}

		for _, deployLine := range deployPipeSplit {
			trcshDriverConfig.DriverConfig.IsShellSubProcess = false
			os.Args = argsOrig
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.

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
						os.Exit(-1)
					}
					errMessage := err.Error()
					errMessageFiltered := strings.ReplaceAll(errMessage, ":", "-")
					deliverableMsg := fmt.Sprintf("%s encountered errors - %s\n", deployLine, errMessageFiltered)
					go func(dMesg string) {
						trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- dMesg
						trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- cap.CTL_COMPLETE
					}(deliverableMsg)

					atomic.StoreInt64(&trcshDriverConfig.FeatherCtx.RunState, cap.RUN_STARTED)
					content = nil
					goto collaboratorReRun
				} else {
					trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- deployLine
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
	if *dronePtr {
		for {
			completeOnce := false
			if atomic.LoadInt64(&featherCtx.RunState) == cap.RUNNING {
				if !completeOnce {
					go func() {
						trcshDriverConfig.DriverConfig.DeploymentCtlMessageChan <- cap.CTL_COMPLETE
					}()
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
	//Make the arguments in the script -> os.args.

}

func ResetModifier(coreConfig *core.CoreConfig, tokenName string) {
	//Resetting modifier cache to be used again.
	mod, err := helperkv.NewModifierFromCoreConfig(coreConfig, tokenName, coreConfig.EnvBasis, true)
	if err != nil {
		eUtils.CheckError(coreConfig, err, true)
	}
	mod.RemoveFromCache()
}
