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
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/util"
	"github.com/trimble-oss/tierceron/pkg/core/util/hive"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"gopkg.in/yaml.v2"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var gAgentConfig *capauth.AgentConfigs = nil
var gTrcshConfig *capauth.TrcShConfig
var pluginHandler *hive.PluginHandler = nil

var (
	MODE_PERCH_STR string = string([]byte{cap.MODE_PERCH})
)

const (
	YOU_SHALL_NOT_PASS = "you shall not pass"
)

func createLogFile() (*log.Logger, error) {
	logFile := "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"deploy.log" {
		logFile = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
	}
	f, errOpenFile := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if errOpenFile != nil {
		return nil, errOpenFile
	}
	logger := log.New(f, "[DEPLOY]", log.LstdFlags)
	return logger, nil
}

func TrcshInitConfig(env string, region string, pathParam string, outputMemCache bool, logger ...*log.Logger) (*capauth.TrcshDriverConfig, error) {
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

	//Check if logfile passed in - if not call create log method that does following below...
	var logFile *log.Logger
	var err error
	if len(logger) == 0 {
		logFile, err = createLogFile()
		if err != nil {
			return nil, err
		}
	} else {
		logFile = logger[0]
	}

	trcshDriverConfig := &capauth.TrcshDriverConfig{
		DriverConfig: eUtils.DriverConfig{
			CoreConfig: core.CoreConfig{
				IsShell:       true,
				Insecure:      false,
				Env:           env,
				EnvBasis:      eUtils.GetEnvBasis(env),
				Regions:       regions,
				ExitOnFailure: true,
				Log:           logFile,
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
		featherCtx.Log.Printf(msg)
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
func EnableDeployer(env string, region string, token string, trcPath string, secretId *string, approleId *string, outputMemCache bool, deployment string, dronePtr *bool, projectService ...*string) {
	trcshDriverConfig, err := TrcshInitConfig(env, region, "", outputMemCache)
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
	if len(projectService) > 0 && coreopts.BuildOptions.IsKernel() {
		projServ = *projectService[0]
	}

	go ProcessDeploy(trcshDriverConfig.FeatherCtx, trcshDriverConfig, "", deployment, trcPath, projServ, secretId, approleId, false, dronePtr)
}

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.

func CommonMain(envPtr *string, addrPtr *string, envCtxPtr *string,
	secretIDPtr *string,
	appRoleIDPtr *string,
	flagset *flag.FlagSet,
	argLines []string,
	c *eUtils.DriverConfig) error {

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

	if coreopts.BuildOptions.IsKernel() {
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

		trcshDriverConfig, err := TrcshInitConfig(*envPtr, *regionPtr, pathParam, true)
		if err != nil {
			fmt.Printf("trcsh config setup failure: %s\n", err.Error())
			os.Exit(124)
		}

		//Open deploy script and parse it.
		ProcessDeploy(nil, trcshDriverConfig, "", "", *trcPathPtr, *projectServicePtr, secretIDPtr, appRoleIDPtr, true, dronePtr)
	} else {
		logger, err := createLogFile()

		if coreopts.BuildOptions.IsKernel() {
			go deployutil.KernelShutdownWatcher(logger)
		}

		if err != nil {
			fmt.Printf("Error initializing log file: %s\n", err)
		}
		var agentToken string
		var agentEnv string
		var address string
		var deploymentsShard string
		fromWinCred := false

		if coreopts.BuildOptions.IsKernel() {
			// load via new properties and get config values
			data, err := os.ReadFile("config.yml")
			if err != nil {
				logger.Println("Error reading YAML file:", err)
				os.Exit(-1) //do we want to exit???
			}
			// Create an empty map for the YAML data
			var config map[string]interface{}

			// Unmarshal the YAML data into the map
			err = yaml.Unmarshal(data, &config)
			if err != nil {
				logger.Println("Error unmarshaling YAML:", err)
				os.Exit(-1)
			}
			if token, ok := config["agent_token"].(string); ok {
				agentToken = token
			} else {
				logger.Println("Error reading config value")
			}
			if addr, ok := config["vault_addr"].(string); ok {
				address = addr
			} else {
				logger.Println("Error reading config value")
			}
			if env, ok := config["agent_env"].(string); ok {
				agentEnv = env
			} else {
				logger.Println("Error reading config value")
			}
			if deployments, ok := config["deployments"].(string); ok {
				deploymentsShard = deployments
			} else {
				logger.Println("Error reading config value")
			}
		} else {
			agentToken = ""
			if eUtils.IsWindows() {
				agentCred, err := wincred.GetGenericCredential("AGENT_TOKEN")
				if err != nil {
					fmt.Println("Error loading authentication from Credential Manager")
					logger.Println("Error loading authentication from Credential Manager")
				} else {
					agentToken = string(agentCred.CredentialBlob)
					fromWinCred = true
				}
			} else {
				agentToken = os.Getenv("AGENT_TOKEN")
			}
			agentEnv = os.Getenv("AGENT_ENV")
			address = os.Getenv("VAULT_ADDR")

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
					logger.Printf("drone trcsh requires a %s.\n", deploymentsKey)
					os.Exit(-1)
				}
			}
		}

		if len(agentToken) == 0 && !eUtils.IsWindows() {
			fmt.Println("drone trcsh requires AGENT_TOKEN.")
			logger.Println("drone trcsh requires AGENT_TOKEN.")
			os.Exit(-1)
		}

		if len(agentEnv) == 0 {
			fmt.Println("drone trcsh requires AGENT_ENV.")
			logger.Println("drone trcsh requires AGENT_ENV.")
			os.Exit(-1)
		}

		if len(*envPtr) > 0 {
			agentEnv = *envPtr
		}

		if len(address) == 0 {
			fmt.Println("drone trcsh requires VAULT_ADDR address.")
			logger.Println("drone trcsh requires VAULT_ADDR address.")
			os.Exit(-1)
		}

		if err := capauth.ValidateVhost(address, "https://", false, logger); err != nil {
			fmt.Printf("drone trcsh requires supported VAULT_ADDR address: %s\n", err.Error())
			logger.Printf("drone trcsh requires supported VAULT_ADDR address: %s\n", err.Error())
			os.Exit(124)
		}

		if coreopts.BuildOptions.IsKernel() {
			hostname := os.Getenv("HOSTNAME")
			id := 0

			if len(hostname) == 0 {
				logger.Println("Looking up set entry by host")
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
				logger.Println("Stateful set enabled")

				// spectrum-aggregator-snapshot-<pool>
				hostParts := strings.Split(hostname, "-")
				id, err = strconv.Atoi(hostParts[1])
				if err != nil {
					id = 0
				}
				logger.Printf("Starting Stateful trcshk with set entry id: %d\n", id)
			}
			if id > 0 {
				agentEnv = fmt.Sprintf("%s-%d", agentEnv, id)
			}
		}

		memprotectopts.MemProtect(nil, &agentToken)
		memprotectopts.MemProtect(nil, &address)
		shutdown := make(chan bool)

		fmt.Printf("drone trcsh beginning new agent configuration sequence.\n")
		logger.Printf("drone trcsh beginning new agent configuration sequence.\n")
		// Preload agent synchronization configs...
		var errAgentLoad error
	ValidateAgent:
		gAgentConfig, gTrcshConfig, errAgentLoad = capauth.NewAgentConfig(address,
			agentToken,
			agentEnv, deployCtlAcceptRemoteNoTimeout, nil, true, logger, dronePtr)
		if errAgentLoad != nil {
			// check os.env for another token
			if agentToken != os.Getenv("AGENT_TOKEN") && eUtils.IsWindows() {
				agentToken = os.Getenv("AGENT_TOKEN")
				fromWinCred = false
				goto ValidateAgent
			} else {
				fmt.Printf("drone trcsh agent bootstrap agent config failure: %s\n", errAgentLoad.Error())
				logger.Printf("drone trcsh agent bootstrap agent config failure: %s\n", errAgentLoad.Error())
				os.Exit(124)
			}
		}

		fmt.Println("Drone trcsh agent bootstrap successful.")
		logger.Println("Drone trcsh agent bootstrap successful.")

		if eUtils.IsWindows() {
			if !fromWinCred {
				// migrate token to wincred
				cred := wincred.NewGenericCredential("AGENT_TOKEN")
				cred.CredentialBlob = []byte(agentToken)
				err := cred.Write()
				if err != nil {
					fmt.Printf("Error migrating updated token: %s\n", err)
					logger.Printf("Error migrating updated token: %s\n", err)
				}
			}
			//delete os.env token
			if os.Getenv("AGENT_TOKEN") != "" {
				command := exec.Command("cmd", "/C", "setx", "/M", "AGENT_TOKEN", "UNSET")
				_, err := command.CombinedOutput()
				if err != nil {
					fmt.Println(err)
					logger.Println(err)
				}
			}
		}

		fmt.Printf("drone trcsh beginning initialization sequence.\n")
		logger.Printf("drone trcsh beginning initialization sequence.\n")
		// Initialize deployers.
		trcshDriverConfig, err := TrcshInitConfig(*gAgentConfig.Env, *regionPtr, "", true, logger)
		if err != nil {
			fmt.Printf("drone trcsh agent bootstrap init config failure: %s\n", err.Error())
			logger.Printf("drone trcsh agent bootstrap init config failure: %s\n", err.Error())
			os.Exit(124)
		}

		// Validate drone sha path
		pluginConfig := make(map[string]interface{})
		pluginConfig["vaddress"] = address
		pluginConfig["token"] = agentToken
		pluginConfig["env"] = trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
		if eUtils.IsWindows() {
			pluginConfig["plugin"] = "trcsh.exe"
		} else if coreopts.BuildOptions.IsKernel() {
			pluginConfig["plugin"] = "trcshk"
		} else {
			pluginConfig["plugin"] = "trcsh"
		}

		_, mod, vault, err := eUtils.InitVaultModForPlugin(pluginConfig, logger)
		if err != nil {
			fmt.Printf("Problem initializing mod: %s\n", err)
			logger.Printf("Problem initializing mod: %s\n", err)
		}
		if vault != nil {
			defer vault.Close()
		}

		isValid, err := trcshauth.ValidateTrcshPathSha(mod, pluginConfig, logger)
		if err != nil || !isValid {
			fmt.Printf("Error obtaining authorization components: %s\n", err)
			os.Exit(124)
		}

		if coreopts.BuildOptions.IsKernel() && pluginHandler == nil {
			pluginHandler = &hive.PluginHandler{
				IsRunning: false,
				Services:  &[]string{deploymentsShard},
			}
		}

		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Completed bootstrapping and continuing to initialize services.")
		trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfig = *gTrcshConfig.ConfigRole
		trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress = *gTrcshConfig.VaultAddress

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

		for _, deployment := range deployments {
			EnableDeployer(*gAgentConfig.Env, *regionPtr, deployment, *trcPathPtr, secretIDPtr, appRoleIDPtr, false, deployment, dronePtr, projectServicePtr)
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
	isAgentToken bool,
	token string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) error {
	*configCount -= 1
	trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfig = "config.yml"
	trcshDriverConfig.DriverConfig.FileFilter = nil
	trcshDriverConfig.DriverConfig.CoreConfig.WantCerts = false
	trcshDriverConfig.DriverConfig.IsShellSubProcess = true
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner init: %s\n", control)

	if trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress == "" {
		trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress = *gTrcshConfig.VaultAddress
	}
	if trcDeployRoot, ok := trcshDriverConfig.DriverConfig.DeploymentConfig["trcdeployroot"]; ok {
		trcshDriverConfig.DriverConfig.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
		trcshDriverConfig.DriverConfig.EndDir = trcDeployRoot.(string)
	} else {
		trcshDriverConfig.DriverConfig.StartDir = []string{"trc_templates"}
		trcshDriverConfig.DriverConfig.EndDir = "."
	}
	configRoleSlice := strings.Split(*gTrcshConfig.ConfigRole, ":")
	tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	tokenConfig := ""
	envDefaultPtr := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	var err error
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Role runner complete: %s\n", control)

	switch control {
	case "trcplgtool":
		tokenConfig := token
		envDefaultPtr = trcshDriverConfig.DriverConfig.CoreConfig.Env
		err = trcplgtoolbase.CommonMain(&envDefaultPtr, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &tokenConfig, &gTrcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, deployArgLines, trcshDriverConfig, pluginHandler)
	case "trcconfig":
		if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" || trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "staging" || trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "prod" ||
			trcshDriverConfig.DriverConfig.CoreConfig.Env == "itdev" || trcshDriverConfig.DriverConfig.CoreConfig.Env == "staging" || trcshDriverConfig.DriverConfig.CoreConfig.Env == "prod" {
			trcshDriverConfig.DriverConfig.OutputMemCache = false
			// itdev, staging, and prod always key off TRC_ENV stored in trcshDriverConfig.DriverConfig.CoreConfig.Env.
			envDefaultPtr = trcshDriverConfig.DriverConfig.CoreConfig.Env
			tokenName = "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.Env
		}
		err = trcconfigbase.CommonMain(&envDefaultPtr, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &tokenConfig, &gTrcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, deployArgLines, &trcshDriverConfig.DriverConfig)
	case "trcsub":
		trcshDriverConfig.DriverConfig.EndDir = trcshDriverConfig.DriverConfig.EndDir + "/trc_templates"
		err = trcsubbase.CommonMain(&envDefaultPtr, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &gTrcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], nil, deployArgLines, &trcshDriverConfig.DriverConfig)
	}
	ResetModifier(&trcshDriverConfig.DriverConfig.CoreConfig) //Resetting modifier cache to avoid token conflicts.

	if !isAgentToken {
		token = ""
		trcshDriverConfig.DriverConfig.CoreConfig.Token = token
	}
	return err
}

func processPluginCmds(trcKubeDeploymentConfig **kube.TrcKubeConfig,
	onceKubeInit *sync.Once,
	PipeOS billy.File,
	env string,
	region string,
	trcshDriverConfig *capauth.TrcshDriverConfig,
	control string,
	isAgentToken bool,
	token string,
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
		ResetModifier(&trcshDriverConfig.DriverConfig.CoreConfig) //Resetting modifier cache to avoid token conflicts.
		trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfig = "configpub.yml"
		trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis = env
		trcshDriverConfig.DriverConfig.IsShellSubProcess = true
		trcshDriverConfig.DriverConfig.CoreConfig.WantCerts = true

		pubRoleSlice := strings.Split(*gTrcshConfig.PubRole, ":")
		tokenName := "vault_pub_token_" + env
		tokenPub := ""
		pubEnv := env

		trcinitbase.CommonMain(&pubEnv, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &tokenPub, &gTrcshConfig.EnvContext, &pubRoleSlice[1], &pubRoleSlice[0], &tokenName, &trcshDriverConfig.DriverConfig.CoreConfig.WantCerts, nil, deployArgLines, &trcshDriverConfig.DriverConfig)
		ResetModifier(&trcshDriverConfig.DriverConfig.CoreConfig) //Resetting modifier cache to avoid token conflicts.
		if !isAgentToken {
			token = ""
			trcshDriverConfig.DriverConfig.CoreConfig.Token = token
		}
	case "trcpub":
		ResetModifier(&trcshDriverConfig.DriverConfig.CoreConfig) //Resetting modifier cache to avoid token conflicts.
		trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfig = "configpub.yml"
		trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis = env
		trcshDriverConfig.DriverConfig.IsShellSubProcess = true

		pubRoleSlice := strings.Split(*gTrcshConfig.PubRole, ":")
		tokenName := "vault_pub_token_" + env
		tokenPub := ""
		pubEnv := env

		trcpubbase.CommonMain(&pubEnv, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &tokenPub, &gTrcshConfig.EnvContext, &pubRoleSlice[1], &pubRoleSlice[0], &tokenName, nil, deployArgLines, &trcshDriverConfig.DriverConfig)
		ResetModifier(&trcshDriverConfig.DriverConfig.CoreConfig) //Resetting modifier cache to avoid token conflicts.
		if !isAgentToken {
			token = ""
			trcshDriverConfig.DriverConfig.CoreConfig.Token = token
		}
	case "trcconfig":
		err := roleBasedRunner(region, trcshDriverConfig, control, isAgentToken, token, argsOrig, deployArgLines, configCount)
		if err != nil {
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
			if gTrcshConfig == nil || gTrcshConfig.VaultAddress == nil || gTrcshConfig.Token == nil {
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
			gAgentConfig, _, errAgentLoad = capauth.NewAgentConfig(*gTrcshConfig.VaultAddress,
				*gTrcshConfig.Token,
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

		err := roleBasedRunner(region, trcshDriverConfig, control, isAgentToken, *gTrcshConfig.Token, argsOrig, deployArgLines, configCount)
		if err != nil {
			os.Exit(1)
		}

	case "kubectl":
		onceKubeInit.Do(func() {
			var kubeInitErr error
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Setting up kube config")
			*trcKubeDeploymentConfig, kubeInitErr = kube.InitTrcKubeConfig(gTrcshConfig, &trcshDriverConfig.DriverConfig.CoreConfig)
			if kubeInitErr != nil {
				fmt.Println(kubeInitErr)
				return
			}
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Setting kube config setup complete")
		})
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Preparing for kubectl")
		(*(*trcKubeDeploymentConfig)).PipeOS = PipeOS

		kubectlErrChan := make(chan error, 1)

		go func(dConfig *eUtils.DriverConfig) {
			dConfig.CoreConfig.Log.Println("Executing kubectl")
			kubectlErrChan <- kube.KubeCtl(*trcKubeDeploymentConfig, dConfig)
		}(&trcshDriverConfig.DriverConfig)

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
	isAgentToken bool,
	token string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) error {

	err := roleBasedRunner(region, trcshDriverConfig, control, isAgentToken, token, argsOrig, deployArgLines, configCount)
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
	token string,
	deployment string,
	trcPath string,
	projectServicePtr string,
	secretId *string,
	approleId *string,
	outputMemCache bool,
	dronePtr *bool) {
	// Verify Billy implementation
	configMemFs := trcshDriverConfig.DriverConfig.MemFs.(*trcshMemFs.TrcshMemFs)

	isAgentToken := false
	if token != "" {
		isAgentToken = true
	}
	pwd, _ := os.Getwd()
	var content []byte

	if trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis == "itdev" {
		trcshDriverConfig.DriverConfig.OutputMemCache = false
	}
	fmt.Println("Logging initialized.")
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Logging initialized for env:%s\n", trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis)

	var err error
	vaultAddress, err := trcshauth.TrcshVAddress(featherCtx, gAgentConfig, trcshDriverConfig)
	if err != nil || len(*vaultAddress) == 0 {
		fmt.Println("Auth phase 0 failure")
		if err != nil {
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Error: %s\n", err.Error())
		}
		os.Exit(-1)
	}
	trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress = *vaultAddress
	// Chewbacca: scrub before checkin
	// This data is generated by TrcshAuth
	// cToken := ""
	// configRole := os.Getenv("CONFIG_ROLE")
	// pubRole := ""
	// fileBytes, _ := os.ReadFile("")
	// kc := base64.StdEncoding.EncodeToString(fileBytes)
	// gTrcshConfig = &capauth.TrcShConfig{Env: "dev",
	// 	EnvContext: "dev",
	// 	CToken:     &cToken,
	// 	ConfigRole: &configRole,
	// 	PubRole:    &pubRole,
	// 	KubeConfig: &kc,
	// }
	// trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress = ""
	// gTrcshConfig.VaultAddress = &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress
	// trcshDriverConfig.DriverConfig.CoreConfig.Token = ""
	//	Chewbacca: end scrub
	trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Auth..")

	trcshEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis
	tokenPtr := new(string)
	authTokenEnv := "azuredeploy"
	appRoleConfig := "deployauth"
	if gAgentConfig != nil && gAgentConfig.AgentToken != nil {
		tokenPtr = gAgentConfig.AgentToken
		appRoleConfig = "none"
	}
	authTokenName := "vault_token_azuredeploy"
	autoErr := eUtils.AutoAuth(&trcshDriverConfig.DriverConfig, secretId, approleId, tokenPtr, &authTokenName, &authTokenEnv, &trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress, &trcshEnvBasis, appRoleConfig, false)
	if autoErr != nil || tokenPtr == nil || *tokenPtr == "" {
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
	mergedVaultAddress := trcshDriverConfig.DriverConfig.CoreConfig.VaultAddress
	mergedEnvBasis := trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

	if len(mergedVaultAddress) == 0 {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil && gTrcshConfig.VaultAddress != nil {
			mergedVaultAddress = *gTrcshConfig.VaultAddress
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

	if coreopts.IsKernel() || ((len(os.Args) > 1) && len(trcPath) > 0) && !strings.Contains(pwd, "TrcDeploy") {
		// Generate trc code...
		trcshDriverConfig.DriverConfig.CoreConfig.Log.Println("Preload setup")
		configRoleSlice := strings.Split(*gTrcshConfig.ConfigRole, ":")
		tokenName := "config_token_" + trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis

		if coreopts.IsKernel() {
			pluginMap := map[string]interface{}{"pluginName": deployment}

			var configToken string

			autoErr := eUtils.AutoAuth(&trcshDriverConfig.DriverConfig, &configRoleSlice[1], &configRoleSlice[0], &configToken, &tokenName, &mergedEnvBasis, &mergedVaultAddress, &mergedEnvBasis, trcshDriverConfig.DriverConfig.CoreConfig.AppRoleConfig, false)
			if autoErr != nil {
				fmt.Printf("Kernel Missing auth components: %s.\n", deployment)
				return
			}
			if memonly.IsMemonly() {
				memprotectopts.MemUnprotectAll(nil)
				memprotectopts.MemProtect(nil, tokenPtr)
			}

			mod, err := helperkv.NewModifier(trcshDriverConfig.DriverConfig.CoreConfig.Insecure, *tokenPtr, mergedVaultAddress, mergedEnvBasis, nil, true, trcshDriverConfig.DriverConfig.CoreConfig.Log)
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

			err := trcsubbase.CommonMain(&trcshDriverConfig.DriverConfig.CoreConfig.EnvBasis, &mergedVaultAddress,
				&mergedEnvBasis, &configRoleSlice[1], &configRoleSlice[0], nil, []string{"trcsh", "-templatePaths=" + templatePathsPtr}, &trcshDriverConfig.DriverConfig)
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

		configErr := trcconfigbase.CommonMain(&envConfig, &mergedVaultAddress, &token, &mergedEnvBasis, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, []string{"trcsh"}, &trcshDriverConfig.DriverConfig)
		if configErr != nil {
			fmt.Println("Preload failed.  Couldn't find required resource.")
			trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Preload Error %s\n", configErr.Error())
			os.Exit(123)
		}
		ResetModifier(&trcshDriverConfig.DriverConfig.CoreConfig) //Resetting modifier cache to avoid token conflicts.
		if !isAgentToken {
			token = ""
			trcshDriverConfig.DriverConfig.CoreConfig.Token = token
		}

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

		if !isAgentToken {
			token = ""
			trcshDriverConfig.DriverConfig.CoreConfig.Token = token
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
	if featherCtx != nil && !coreopts.BuildOptions.IsKernel() {
		// featherCtx initialization is delayed for the self contained deployments (kubernetes, etc...)
		for {
			if atomic.LoadInt64(&featherCtx.RunState) == cap.RESETTING {
				break
			} else {
				acceptRemote(featherCtx, cap.FEATHER_CTL, "")
			}
		}

		if content == nil {
			content, err = deployutil.LoadPluginDeploymentScript(trcshDriverConfig, gTrcshConfig, pwd)
			if err != nil {
				trcshDriverConfig.DriverConfig.CoreConfig.Log.Printf("Failure to load deployment: %s\n", trcshDriverConfig.DriverConfig.DeploymentConfig["trcplugin"])
				time.Sleep(time.Minute)
				goto collaboratorReRun
			}
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
					isAgentToken,
					token,
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
					isAgentToken,
					token,
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

func ResetModifier(coreConfig *core.CoreConfig) {
	//Resetting modifier cache to be used again.
	mod, err := helperkv.NewModifierFromCoreConfig(coreConfig, coreConfig.EnvBasis, true)
	if err != nil {
		eUtils.CheckError(coreConfig, err, true)
	}
	mod.RemoveFromCache()
}
