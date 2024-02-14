package trcshbase

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/trimble-oss/tierceron-hat/cap"
	captiplib "github.com/trimble-oss/tierceron-hat/captip/captiplib"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/deployutil"
	kube "github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcsh/trcshauth"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/core/util"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var gAgentConfig *capauth.AgentConfigs = nil
var gTrcshConfig *capauth.TrcShConfig

var (
	MODE_PERCH_STR string = string([]byte{cap.MODE_PERCH})
)

const (
	YOU_SHALL_NOT_PASS = "you shall not pass"
)

func TrcshInitConfig(env string, region string, outputMemCache bool) (*eUtils.DriverConfig, error) {
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

	logFile := "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"deploy.log" {
		logFile = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "deploy.log"
	}
	f, errOpenFile := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if errOpenFile != nil {
		return nil, errOpenFile
	}
	logger := log.New(f, "[DEPLOY]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: true,
		Env:               env,
		EnvRaw:            env,
		Log:               logger,
		IsShell:           true,
		IsShellSubProcess: false,
		OutputMemCache:    outputMemCache,
		MemFs:             memfs.New(),
		Regions:           regions,
		ExitOnFailure:     true}
	return config, nil
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
func EnableDeployer(env string, region string, token string, trcPath string, secretId *string, approleId *string, outputMemCache bool, deployment string) {
	config, err := TrcshInitConfig(env, region, outputMemCache)
	if err != nil {
		fmt.Printf("Initialization setup error: %s\n", err.Error())
	}
	if len(deployment) > 0 {
		// Set the name of the plugin to deploy in "trcplugin"
		// Used later by codedeploy
		config.DeploymentConfig = map[string]interface{}{"trcplugin": deployment}
		config.DeploymentCtlMessageChan = make(chan string, 5)
		fmt.Printf("Starting deployer: %s\n", deployment)
		config.Log.Printf("Starting deployer: %s\n", deployment)
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
	config.FeatherCtx = captiplib.FeatherCtlInit(interruptChan,
		&localHostAddr,
		gAgentConfig.EncryptPass,
		gAgentConfig.EncryptSalt,
		gAgentConfig.HostAddr,
		gAgentConfig.HandshakeCode,
		&sessionIdentifier, /*Session identifier */
		&env,
		deployerAcceptRemoteNoTimeout,
		deployerInterrupted)
	config.FeatherCtx.Log = config.Log
	// featherCtx initialization is delayed for the self contained deployments (kubernetes, etc...)
	atomic.StoreInt64(&config.FeatherCtx.RunState, cap.RUN_STARTED)

	go captiplib.FeatherCtlEmitter(config.FeatherCtx, config.DeploymentCtlMessageChan, deployerEmote, nil)

	go ProcessDeploy(config.FeatherCtx, config, "", deployment, trcPath, secretId, approleId, false)
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
		flagset.String("secretID", "", "Public app role ID")
		flagset.String("appRoleID", "", "Secret app role ID")
	}

	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	eUtils.InitHeadless(true)
	var regionPtr, trcPathPtr *string
	// Initiate signal handling.
	var ic chan os.Signal = make(chan os.Signal, 5)

	if !eUtils.IsWindows() {
		signal.Notify(ic, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT)
	} else {
		signal.Notify(ic, os.Interrupt)
	}
	go func() {
		x := <-ic
		interruptChan <- x
	}()

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
		regionPtr = flagset.String("region", "", "Region to be processed")  //If this is blank -> use context otherwise override context.
		trcPathPtr = flagset.String("c", "", "Optional script to execute.") //If this is blank -> use context otherwise override context.
		flagset.Parse(argLines[1:])

		if len(*appRoleIDPtr) == 0 {
			*appRoleIDPtr = os.Getenv("DEPLOY_ROLE")
		}

		if len(*secretIDPtr) == 0 {
			*secretIDPtr = os.Getenv("DEPLOY_SECRET")
		}
		memprotectopts.MemProtect(nil, secretIDPtr)
		memprotectopts.MemProtect(nil, appRoleIDPtr)

		config, err := TrcshInitConfig(*envPtr, *regionPtr, true)
		if err != nil {
			fmt.Printf("trcsh config setup failure: %s\n", err.Error())
			os.Exit(-1)
		}

		//Open deploy script and parse it.
		ProcessDeploy(nil, config, "", "", *trcPathPtr, secretIDPtr, appRoleIDPtr, true)
	} else {
		deploymentsShard := os.Getenv("DEPLOYMENTS")
		agentToken := os.Getenv("AGENT_TOKEN")
		agentEnv := os.Getenv("AGENT_ENV")
		address := os.Getenv("VAULT_ADDR")

		regionPtr = flagset.String("region", "", "Region to be processed")  //If this is blank -> use context otherwise override context.
		trcPathPtr = flagset.String("c", "", "Optional script to execute.") //If this is blank -> use context otherwise override context.
		flagset.Parse(argLines[1:])

		if len(deploymentsShard) == 0 {
			fmt.Println("trcsh on windows requires a DEPLOYMENTS.")
			os.Exit(-1)
		}

		if len(agentToken) == 0 {
			fmt.Println("trcsh on windows requires AGENT_TOKEN.")
			os.Exit(-1)
		}

		if len(agentEnv) == 0 {
			fmt.Println("trcsh on windows requires AGENT_ENV.")
			os.Exit(-1)
		}

		if len(*envPtr) > 0 {
			agentEnv = *envPtr
		}

		if len(address) == 0 {
			fmt.Println("trcsh on windows requires VAULT_ADDR address.")
			os.Exit(-1)
		}
		if err := capauth.ValidateVhost(address, "https://"); err != nil {
			fmt.Printf("trcsh on windows requires supported VAULT_ADDR address: %s\n", err.Error())
			os.Exit(-1)
		}
		memprotectopts.MemProtect(nil, &agentToken)
		memprotectopts.MemProtect(nil, &address)
		shutdown := make(chan bool)

		fmt.Printf("trcsh beginning new agent configuration sequence.\n")
		// Preload agent synchronization configs...
		var errAgentLoad error
		var trcshConfig *capauth.TrcShConfig
		gAgentConfig, trcshConfig, errAgentLoad = capauth.NewAgentConfig(address,
			agentToken,
			agentEnv, deployCtlAcceptRemoteNoTimeout, nil)
		if errAgentLoad != nil {
			fmt.Println("trcsh agent bootstrap failure.")
			os.Exit(-1)
		}

		fmt.Printf("trcsh beginning initialization sequence.\n")
		// Initialize deployers.
		config, err := TrcshInitConfig(*gAgentConfig.Env, *regionPtr, true)
		if err != nil {
			fmt.Println("trcsh agent bootstrap failure.")
			os.Exit(-1)
		}
		config.AppRoleConfig = *trcshConfig.ConfigRole
		config.VaultAddress = *trcshConfig.VaultAddress
		serviceDeployments, err := deployutil.GetDeployers(config)
		if err != nil {
			fmt.Println("trcsh agent bootstrap failure.")
			os.Exit(-1)
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

		for _, deployment := range deployments {
			EnableDeployer(*gAgentConfig.Env, *regionPtr, deployment, *trcPathPtr, secretIDPtr, appRoleIDPtr, false, deployment)
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
		os.Exit(1)
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
		os.Exit(1)
	}
	return result, resultError
}

func interruptFun(featherCtx *cap.FeatherContext, tickerInterrupt *time.Ticker) {
	select {
	case <-tickerInterrupt.C:
		if len(featherCtx.InterruptChan) > 0 {
			cap.FeatherCtlEmit(featherCtx, MODE_PERCH_STR, *featherCtx.SessionIdentifier, true)
			os.Exit(1)
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
		os.Exit(-1)
	}

	return nil
}

func roleBasedRunner(env string,
	trcshConfig *capauth.TrcShConfig,
	region string,
	config *eUtils.DriverConfig,
	control string,
	agentToken bool,
	token string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) error {
	*configCount -= 1
	config.AppRoleConfig = "config.yml"
	config.FileFilter = nil
	config.EnvRaw = env
	config.WantCerts = false
	config.IsShellSubProcess = true
	if config.VaultAddress == "" {
		config.VaultAddress = *gTrcshConfig.VaultAddress
	}
	if trcDeployRoot, ok := config.DeploymentConfig["trcdeployroot"]; ok {
		config.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
		config.EndDir = trcDeployRoot.(string)
	} else {
		config.StartDir = []string{"trc_templates"}
		config.EndDir = "."
	}
	configRoleSlice := strings.Split(*trcshConfig.ConfigRole, ":")
	tokenName := "config_token_" + env
	tokenConfig := ""
	configEnv := env
	var err error

	switch control {
	case "trcplgtool":
		tokenConfig := token
		err = trcplgtoolbase.CommonMain(&configEnv, &config.VaultAddress, &tokenConfig, &trcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, deployArgLines, config)
	case "trcconfig":
		if config.EnvRaw == "itdev" || config.EnvRaw == "staging" || config.EnvRaw == "prod" ||
			config.Env == "itdev" || config.Env == "staging" || config.Env == "prod" {
			config.OutputMemCache = false
		}
		err = trcconfigbase.CommonMain(&configEnv, &config.VaultAddress, &tokenConfig, &trcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, deployArgLines, config)
	case "trcsub":
		config.EndDir = config.EndDir + "/trc_templates"
		err = trcsubbase.CommonMain(&configEnv, &config.VaultAddress, &trcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], nil, deployArgLines, config)
	}
	ResetModifier(config) //Resetting modifier cache to avoid token conflicts.

	if !agentToken {
		token = ""
		config.Token = token
	}
	return err
}

func processPluginCmds(trcKubeDeploymentConfig **kube.TrcKubeConfig,
	onceKubeInit *sync.Once,
	PipeOS billy.File,
	env string,
	trcshConfig *capauth.TrcShConfig,
	region string,
	config *eUtils.DriverConfig,
	control string,
	agentToken bool,
	token string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) {
	switch control {
	case "trcpub":
		ResetModifier(config) //Resetting modifier cache to avoid token conflicts.
		config.AppRoleConfig = "configpub.yml"
		config.EnvRaw = env
		config.IsShellSubProcess = true

		pubRoleSlice := strings.Split(*gTrcshConfig.PubRole, ":")
		tokenName := "vault_pub_token_" + env
		tokenPub := ""
		pubEnv := env

		trcpubbase.CommonMain(&pubEnv, &config.VaultAddress, &tokenPub, &trcshConfig.EnvContext, &pubRoleSlice[1], &pubRoleSlice[0], &tokenName, nil, deployArgLines, config)
		ResetModifier(config) //Resetting modifier cache to avoid token conflicts.
		if !agentToken {
			token = ""
			config.Token = token
		}
	case "trcconfig":
		err := roleBasedRunner(env, trcshConfig, region, config, control, agentToken, token, argsOrig, deployArgLines, configCount)
		if err != nil {
			os.Exit(1)
		}
	case "trcplgtool":
		// Utilize elevated CToken to perform certifications if asked.
		config.FeatherCtlCb = featherCtlCb
		if gAgentConfig == nil {
			var errAgentLoad error
			// Prepare the configuration triggering mechanism.
			// Bootstrap deployment is replaced during callback with the agent name.
			gAgentConfig, _, errAgentLoad = capauth.NewAgentConfig(config.VaultAddress,
				*trcshConfig.CToken,
				config.Env,
				deployCtlAcceptRemote,
				deployCtlInterrupted)
			if errAgentLoad != nil {
				fmt.Printf("Permissions failure.  Incorrect deployment")
				os.Exit(1)
			}
			if gAgentConfig.FeatherContext == nil {
				fmt.Printf("Warning!  Permissions failure.  Incorrect feathering")
			}
			gAgentConfig.InterruptHandlerFunc = deployCtlInterrupted
		}
		config.FeatherCtx = captiplib.FeatherCtlInit(interruptChan,
			gAgentConfig.LocalHostAddr,
			gAgentConfig.EncryptPass,
			gAgentConfig.EncryptSalt,
			gAgentConfig.HostAddr,
			gAgentConfig.HandshakeCode,
			new(string),
			&env,
			deployCtlAcceptRemote,
			deployCtlInterrupted)
		if config.Log != nil {
			config.FeatherCtx.Log = config.Log
		}

		err := roleBasedRunner(env, trcshConfig, region, config, control, agentToken, *trcshConfig.CToken, argsOrig, deployArgLines, configCount)
		if err != nil {
			os.Exit(1)
		}

	case "kubectl":
		onceKubeInit.Do(func() {
			var kubeInitErr error
			config.Log.Println("Setting up kube config")
			*trcKubeDeploymentConfig, kubeInitErr = kube.InitTrcKubeConfig(trcshConfig, config)
			if kubeInitErr != nil {
				fmt.Println(kubeInitErr)
				return
			}
			config.Log.Println("Setting kube config setup complete")
		})
		config.Log.Println("Preparing for kubectl")
		(*(*trcKubeDeploymentConfig)).PipeOS = PipeOS

		kubectlErrChan := make(chan error, 1)

		go func(c *eUtils.DriverConfig) {
			c.Log.Println("Executing kubectl")
			kubectlErrChan <- kube.KubeCtl(*trcKubeDeploymentConfig, c)
		}(config)

		select {
		case <-time.After(15 * time.Second):
			fmt.Println("Agent is not yet ready..")
			config.Log.Println("Timed out waiting for KubeCtl.")
			os.Exit(-1)
		case kubeErr := <-kubectlErrChan:
			if kubeErr != nil {
				config.Log.Println(kubeErr)
				os.Exit(-1)
			}
		}
	}
}

func processWindowsCmds(trcKubeDeploymentConfig *kube.TrcKubeConfig,
	onceKubeInit *sync.Once,
	PipeOS billy.File,
	env string,
	trcshConfig *capauth.TrcShConfig,
	region string,
	config *eUtils.DriverConfig,
	control string,
	agentToken bool,
	token string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) error {

	err := roleBasedRunner(env, trcshConfig, region, config, control, agentToken, token, argsOrig, deployArgLines, configCount)
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
func ProcessDeploy(featherCtx *cap.FeatherContext, config *eUtils.DriverConfig, token string, deployment string, trcPath string, secretId *string, approleId *string, outputMemCache bool) {
	agentToken := false
	if token != "" {
		agentToken = true
	}
	pwd, _ := os.Getwd()
	var content []byte

	if config.EnvRaw == "itdev" {
		config.OutputMemCache = false
	}
	fmt.Println("Logging initialized.")
	config.Log.Printf("Logging initialized for env:%s\n", config.EnvRaw)

	var err error
	vaultAddress, err := trcshauth.TrcshVAddress(featherCtx, gAgentConfig, config)
	if err != nil || len(*vaultAddress) == 0 {
		fmt.Println("Auth phase 0 failure")
		if err != nil {
			config.Log.Printf("Error: %s\n", err.Error())
		}
		os.Exit(-1)
	}
	config.VaultAddress = *vaultAddress
	// Chewbacca: scrub before checkin
	// This data is generated by TrcshAuth
	// cToken := ""
	// configRole := ""
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
	// config.VaultAddress = ""
	// gTrcshConfig.VaultAddress = &config.VaultAddress
	// config.Token = ""
	//	Chewbacca: end scrub
	config.Log.Printf("Auth..")

	trcshEnvRaw := config.EnvRaw
	tokenPtr := new(string)
	authTokenEnv := "azuredeploy"
	appRoleConfig := "deployauth"
	if gAgentConfig != nil && gAgentConfig.AgentToken != nil {
		tokenPtr = gAgentConfig.AgentToken
		appRoleConfig = "none"
	}
	authTokenName := "vault_token_azuredeploy"
	autoErr := eUtils.AutoAuth(config, secretId, approleId, tokenPtr, &authTokenName, &authTokenEnv, &config.VaultAddress, &trcshEnvRaw, appRoleConfig, false)
	if autoErr != nil || tokenPtr == nil || *tokenPtr == "" {
		fmt.Println("Unable to auth.")
		if autoErr != nil {
			fmt.Println(autoErr)
		}
		os.Exit(-1)
	}
	config.Log.Printf("Bootstrap..")
	for {
		if gTrcshConfig == nil || !gTrcshConfig.IsValid(gAgentConfig) {
			// Loop until we have something usable...
			gTrcshConfig, err = trcshauth.TrcshAuth(featherCtx, gAgentConfig, config)
			if err != nil {
				config.Log.Printf(".")
				time.Sleep(time.Second)
				continue
			}
			config.Log.Printf("Auth re-loaded %s\n", config.EnvRaw)
		} else {
			break
		}
	}
	// Chewbacca: Begin dbg comment
	mergedVaultAddress := config.VaultAddress
	mergedEnvRaw := config.EnvRaw

	// Chewbacca: Wipe this next section out 731-739.  Code analysis indicates it's not used.
	if (approleId != nil && len(*approleId) == 0) || (secretId != nil && len(*secretId) == 0) {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil && gTrcshConfig.CToken != nil {
			tokenPtr = gTrcshConfig.CToken
		} else if gAgentConfig.AgentToken != nil {
			tokenPtr = gAgentConfig.AgentToken
		}
	}

	if len(mergedVaultAddress) == 0 {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil && gTrcshConfig.VaultAddress != nil {
			mergedVaultAddress = *gTrcshConfig.VaultAddress
		}
	}

	if len(mergedEnvRaw) == 0 {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil {
			mergedEnvRaw = gTrcshConfig.EnvContext
		}
	}

	// End dbg comment
	if config.IsShell {
		config.Log.Println("Session Authorized")
	} else {
		fmt.Println("Session Authorized")
	}

	if (len(os.Args) > 1 && len(trcPath) > 0) && !strings.Contains(pwd, "TrcDeploy") {
		// Generate trc code...
		config.Log.Println("Preload setup")
		trcPathParts := strings.Split(trcPath, "/")
		config.FileFilter = []string{trcPathParts[len(trcPathParts)-1]}
		configRoleSlice := strings.Split(*gTrcshConfig.ConfigRole, ":")
		if len(configRoleSlice) != 2 {
			fmt.Println("Preload failed.  Couldn't load required resource.")
			config.Log.Printf("Couldn't config auth required resource.")
			os.Exit(-1)
		}

		tokenName := "config_token_" + config.EnvRaw
		config.OutputMemCache = true
		config.StartDir = []string{"trc_templates"}
		config.EndDir = "."
		config.Log.Printf("Preloading path %s env %s\n", trcPath, config.EnvRaw)
		region := ""
		if len(config.Regions) > 0 {
			region = config.Regions[0]
		}

		configErr := trcconfigbase.CommonMain(&config.EnvRaw, &mergedVaultAddress, &token, &mergedEnvRaw, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, []string{"trcsh"}, config)
		if configErr != nil {
			fmt.Println("Preload failed.  Couldn't find required resource.")
			config.Log.Printf("Preload Error %s\n", configErr.Error())
			os.Exit(-1)
		}
		ResetModifier(config) //Resetting modifier cache to avoid token conflicts.
		if !agentToken {
			token = ""
			config.Token = token
		}

		var memFile billy.File
		var memFileErr error

		if memFile, memFileErr = config.MemFs.Open(trcPath); memFileErr == nil {
			// Read the generated .trc code...
			buf := bytes.NewBuffer(nil)
			io.Copy(buf, memFile) // Error handling elided for brevity.
			content = buf.Bytes()
		} else {
			fmt.Println("Error could not find " + trcPath + " for deployment instructions")
		}

		if !agentToken {
			token = ""
			config.Token = token
		}
		if config.EnvRaw == "itdev" || config.EnvRaw == "staging" || config.EnvRaw == "prod" {
			config.OutputMemCache = false
		}
		config.Log.Println("Processing trcshell")
	} else {
		if !strings.Contains(pwd, "TrcDeploy") || len(config.DeploymentConfig) == 0 {
			fmt.Println("Processing manual trcshell")
			if config.EnvRaw == "itdev" {
				content, err = os.ReadFile(pwd + "/deploy/buildtest.trc")
				if err != nil {
					fmt.Println("Error could not find /deploy/buildtest.trc for deployment instructions")
				}
			} else {
				content, err = os.ReadFile(pwd + "/deploy/deploy.trc")
				if err != nil {
					fmt.Println("Error could not find " + pwd + "/deploy/deploy.trc for deployment instructions")
					config.Log.Printf("Error could not find %s/deploy/deploy.trc for deployment instructions", pwd)
				}
			}
		}
	}

collaboratorReRun:
	if featherCtx != nil {
		// featherCtx initialization is delayed for the self contained deployments (kubernetes, etc...)
		for {
			if atomic.LoadInt64(&featherCtx.RunState) == cap.RESETTING {
				break
			} else {
				acceptRemote(featherCtx, cap.FEATHER_CTL, "")
			}
		}

		if content == nil {
			content, err = deployutil.LoadPluginDeploymentScript(config, gTrcshConfig, pwd)
			if err != nil {
				config.Log.Printf("Failure to load deployment: %s\n", config.DeploymentConfig["trcplugin"])
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
		deployPipeline = strings.TrimLeft(deployPipeline, " ")
		if strings.HasPrefix(deployPipeline, "#") || deployPipeline == "" {
			continue
		}
		// Print current process line.
		fmt.Println(deployPipeline)
		deployPipeSplit := strings.Split(deployPipeline, "|")

		if PipeOS, err = config.MemFs.Create("io/STDIO"); err != nil {
			fmt.Println("Failure to open io stream.")
			os.Exit(-1)
		}

		for _, deployLine := range deployPipeSplit {
			config.IsShellSubProcess = false
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
			if eUtils.IsWindows() {
				// Log for traceability.
				config.Log.Println(deployLine)
				region := ""
				if len(config.Regions) > 0 {
					region = config.Regions[0]
				}
				err := processWindowsCmds(
					trcKubeDeploymentConfig,
					&onceKubeInit,
					PipeOS,
					config.Env,
					gTrcshConfig,
					region,
					config,
					control,
					agentToken,
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
						config.DeploymentCtlMessageChan <- dMesg
						config.DeploymentCtlMessageChan <- cap.CTL_COMPLETE
					}(deliverableMsg)

					atomic.StoreInt64(&config.FeatherCtx.RunState, cap.RUN_STARTED)
					content = nil
					goto collaboratorReRun
				} else {
					config.DeploymentCtlMessageChan <- deployLine
				}
			} else {
				config.FeatherCtx = featherCtx
				region := ""
				if len(config.Regions) > 0 {
					region = config.Regions[0]
				}

				processPluginCmds(
					&trcKubeDeploymentConfig,
					&onceKubeInit,
					PipeOS,
					config.Env,
					gTrcshConfig,
					region,
					config,
					control,
					agentToken,
					token,
					argsOrig,
					strings.Split(deployLine, " "),
					&configCount)
			}
		}
	}
	if eUtils.IsWindows() {
		for {
			completeOnce := false
			if atomic.LoadInt64(&featherCtx.RunState) == cap.RUNNING {
				if !completeOnce {
					go func() {
						config.DeploymentCtlMessageChan <- cap.CTL_COMPLETE
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

func ResetModifier(config *eUtils.DriverConfig) {
	//Resetting modifier cache to be used again.
	mod, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.EnvRaw, config.Regions, true, config.Log)
	if err != nil {
		eUtils.CheckError(config, err, true)
	}
	mod.RemoveFromCache()
}
