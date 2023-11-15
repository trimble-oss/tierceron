package main

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
	"syscall"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/capauth"
	"github.com/trimble-oss/tierceron/trcconfigbase"
	vcutils "github.com/trimble-oss/tierceron/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/trcpubbase"
	kube "github.com/trimble-oss/tierceron/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/trcsh/trcshauth"
	"github.com/trimble-oss/tierceron/trcsubbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/trcvault/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/trcvault/util"
	"github.com/trimble-oss/tierceron/utils"
	eUtils "github.com/trimble-oss/tierceron/utils"

	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

var gAgentConfig *capauth.AgentConfigs = nil
var gTrcshConfig *capauth.TrcShConfig

func TrcshInitConfig(env string, region string, outputMemCache bool) (*eUtils.DriverConfig, error) {
	if len(env) == 0 {
		env = os.Getenv("TRC_ENV")
	}
	if len(region) == 0 {
		region = os.Getenv("TRC_REGION")
	}

	regions := []string{}
	if strings.HasPrefix(env, "staging") || strings.HasPrefix(env, "prod") || strings.HasPrefix(env, "dev") {
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

	logFile := "./" + coreopts.GetFolderPrefix(nil) + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.GetFolderPrefix(nil)+"deploy.log" {
		logFile = "./" + coreopts.GetFolderPrefix(nil) + "deploy.log"
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

func ProcessDeployment(env string, region string, token string, trcPath string, secretId *string, approleId *string, outputMemCache bool, deployment string) {
	config, err := TrcshInitConfig(env, region, outputMemCache)
	if err != nil {
		fmt.Printf("Initialization setup error: %s\n", err.Error())
	}

	go func(c0 *eUtils.DriverConfig) {
		for {
		perching:
			var deployDoneChan chan bool
			if featherMode, featherErr := cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
				*gAgentConfig.EncryptSalt,
				*gAgentConfig.HandshakeHostPort,
				*gAgentConfig.HandshakeCode,
				cap.MODE_FLAP, deployment+"."+*gAgentConfig.Env, false, acceptRemote); featherErr == nil && strings.HasPrefix(featherMode, cap.MODE_GAZE) {
				// Lookup trcPath from deployment

				// Process the script....
				// This will feed CtlMessages into the Timeout and CtlMessage subscriber
				go func(c1 *eUtils.DriverConfig) {
					go func(c2 *eUtils.DriverConfig) {
						// Timeout and CtlMessage subscriber
						select {
						case <-deployDoneChan:
							break
						case <-time.After(120 * time.Second):
							ctlMsg := "Deployment timed out after 120 seconds"
							cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
								*gAgentConfig.EncryptSalt,
								*gAgentConfig.HandshakeHostPort,
								*gAgentConfig.HandshakeCode,
								cap.MODE_PERCH+"_"+ctlMsg, deployment+"."+*gAgentConfig.Env, true, acceptRemote)
							c2.DeploymentCtlMessage <- capauth.TrcCtlComplete
							break
						}
						for range time.After(120 * time.Second) {
						}
					}(c1)

					ProcessDeploy(c1, region, "", deployment, trcPath, secretId, approleId, false)
				}(c0)

				for modeCtl := range c0.DeploymentCtlMessage {
					flapMode := cap.MODE_FLAP + "_" + modeCtl
					ctlFlapMode := flapMode
					var err error = errors.New("init")

					for {
						if err == nil && ctlFlapMode == cap.MODE_PERCH {
							// Acknowledge perching...
							cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
								*gAgentConfig.EncryptSalt,
								*gAgentConfig.HandshakeHostPort,
								*gAgentConfig.HandshakeCode,
								cap.MODE_PERCH, deployment+"."+*gAgentConfig.Env, true, acceptRemote)
							ctlFlapMode = cap.MODE_PERCH
							goto perching
						}

						if err == nil && flapMode != ctlFlapMode {
							// Flap, Gaze, etc...
							interruptFun(twoHundredMilliInterruptTicker)
							break
						} else {
							callFlap := flapMode
							if err == nil {
								interruptFun(twoHundredMilliInterruptTicker)
							} else {
								if err.Error() != "init" {
									interruptFun(secondInterruptTicker)
								}
							}
							ctlFlapMode, err = cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
								*gAgentConfig.EncryptSalt,
								*gAgentConfig.HandshakeHostPort,
								*gAgentConfig.HandshakeCode,
								callFlap, deployment+"."+*gAgentConfig.Env, true, acceptRemote)
						}
					}
					if modeCtl == capauth.TrcCtlComplete {
						// Only exit with TrcCtlComplete.
						cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
							*gAgentConfig.EncryptSalt,
							*gAgentConfig.HandshakeHostPort,
							*gAgentConfig.HandshakeCode,
							cap.MODE_GLIDE, deployment+"."+*gAgentConfig.Env, true, acceptRemote)
						go func() { deployDoneChan <- true }()
						goto deploycomplete
					}
				}
			} else {
				interruptFun(fiveSecondInterruptTicker)
			}
		deploycomplete:
		}
	}(config)
}

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	eUtils.InitHeadless(true)
	fmt.Println("trcsh Version: " + "1.23")
	var envPtr, regionPtr, trcPathPtr, appRoleIDPtr, secretIDPtr *string

	if !utils.IsWindows() {
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
				// Initiate signal handling.
				var ic chan os.Signal = make(chan os.Signal, 3)
				signal.Notify(ic, os.Interrupt, syscall.SIGTERM)
				go func() {
					x := <-ic
					interruptChan <- x
				}()
			}
		}
		envPtr = flag.String("env", "", "Environment to be processed")   //If this is blank -> use context otherwise override context.
		regionPtr = flag.String("region", "", "Region to be processed")  //If this is blank -> use context otherwise override context.
		trcPathPtr = flag.String("c", "", "Optional script to execute.") //If this is blank -> use context otherwise override context.
		appRoleIDPtr = flag.String("appRoleID", "", "Public app role ID")
		secretIDPtr = flag.String("secretID", "", "App role secret")
		flag.Parse()

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
		ProcessDeploy(config, *regionPtr, "", "", *trcPathPtr, secretIDPtr, appRoleIDPtr, true)
	} else {
		gAgentConfig = &capauth.AgentConfigs{}
		deployments := os.Getenv("DEPLOYMENTS")
		agentToken := os.Getenv("AGENT_TOKEN")
		agentEnv := os.Getenv("AGENT_ENV")
		address := os.Getenv("VAULT_ADDR")

		regionPtr = flag.String("region", "", "Region to be processed")  //If this is blank -> use context otherwise override context.
		trcPathPtr = flag.String("c", "", "Optional script to execute.") //If this is blank -> use context otherwise override context.
		appRoleIDPtr = flag.String("appRoleID", "", "Public app role ID")
		secretIDPtr = flag.String("secretID", "", "App role secret")
		flag.Parse()

		if len(deployments) == 0 {
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

		if len(address) == 0 {
			fmt.Println("trcsh on windows requires VAULT_ADDR address.")
			os.Exit(-1)
		}

		memprotectopts.MemProtect(nil, &agentToken)
		memprotectopts.MemProtect(nil, &address)
		shutdown := make(chan bool)

		// Preload agent synchronization configs
		gAgentConfig.LoadConfigs(address, agentToken, deployments, agentEnv)

		deploymentsSlice := strings.Split(deployments, ",")
		for _, deployment := range deploymentsSlice {
			ProcessDeployment(*gAgentConfig.Env, *regionPtr, deployment, *trcPathPtr, secretIDPtr, appRoleIDPtr, false, deployment)
		}

		<-shutdown
	}
}

var interruptChan chan os.Signal = make(chan os.Signal, 5)
var twoHundredMilliInterruptTicker *time.Ticker = time.NewTicker(200 * time.Millisecond)
var secondInterruptTicker *time.Ticker = time.NewTicker(time.Second)
var multiSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 3)
var fiveSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 5)
var fifteenSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 5)
var thirtySecondInterruptTicker *time.Ticker = time.NewTicker(time.Second * 5)

func acceptInterruptFun(tickerContinue *time.Ticker, tickerBreak *time.Ticker, tickerInterrupt *time.Ticker) (bool, error) {
	select {
	case <-interruptChan:
		cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
			*gAgentConfig.EncryptSalt,
			*gAgentConfig.HandshakeHostPort,
			*gAgentConfig.HandshakeCode,
			cap.MODE_PERCH, *gAgentConfig.Deployments+"."+*gAgentConfig.Env, true, nil)
		os.Exit(1)
	case <-tickerContinue.C:
		// don't break... continue...
		return false, nil
	case <-tickerBreak.C:
		// break and continue
		return true, nil
	case <-tickerInterrupt.C:
		// full stop
		return true, errors.New("you shall not pass")
	}
	return true, errors.New("not possible")
}

func interruptFun(tickerInterrupt *time.Ticker) {
	select {
	case <-interruptChan:
		cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
			*gAgentConfig.EncryptSalt,
			*gAgentConfig.HandshakeHostPort,
			*gAgentConfig.HandshakeCode,
			cap.MODE_PERCH, *gAgentConfig.Deployments+"."+*gAgentConfig.Env, true, nil)
		os.Exit(1)
	case <-tickerInterrupt.C:
	}
}

// acceptRemote - hook for instrumenting
func acceptRemote(mode int, remote string) (bool, error) {
	if mode == cap.FEATHER_CTL {
		return acceptInterruptFun(multiSecondInterruptTicker, fifteenSecondInterruptTicker, thirtySecondInterruptTicker)
	}
	return true, nil
}

func featherCtlCb(agentName string) error {

	if gAgentConfig == nil {
		return errors.New("incorrect agent initialization")
	} else {
		gAgentConfig.Deployments = &agentName
	}
	flapMode := cap.MODE_GAZE
	ctlFlapMode := flapMode
	var err error = errors.New("init")

	for {
		if err == nil && ctlFlapMode == cap.MODE_GLIDE {
			if err != nil {
				fmt.Printf("\nDeployment error.\n")
			} else {
				fmt.Printf("\nDeployment complete.\n")
			}
			os.Exit(0)
		} else {
			callFlap := flapMode
			if err == nil {
				if strings.HasPrefix(ctlFlapMode, cap.MODE_FLAP) {
					ctl := strings.Split(ctlFlapMode, "_")
					if len(ctl) > 1 && ctl[1] != capauth.TrcCtlComplete {
						fmt.Printf("%s\n", ctl[1])
					}
					callFlap = cap.MODE_GAZE
				} else {
					callFlap = cap.MODE_GAZE
				}
				interruptFun(twoHundredMilliInterruptTicker)
			} else {
				if err.Error() != "init" {
					interruptFun(multiSecondInterruptTicker)
					callFlap = cap.MODE_GAZE
				}
			}
			ctlFlapMode, err = cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
				*gAgentConfig.EncryptSalt,
				*gAgentConfig.HandshakeHostPort,
				*gAgentConfig.HandshakeCode,
				callFlap, agentName+"."+*gAgentConfig.Env, true, acceptRemote)
		}
	}

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
	if *configCount != 0 { //This is to keep result channel open - closes on the final config call of the script.
		config.IsShellConfigComplete = false
	} else {
		config.IsShellConfigComplete = true
	}
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
			gAgentConfig = &capauth.AgentConfigs{}
			gAgentConfig.LoadConfigs(config.VaultAddress, *trcshConfig.CToken, "bootstrap", "dev") // Feathering always in dev environmnent.
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
func ProcessDeploy(config *eUtils.DriverConfig, region string, token string, deployment string, trcPath string, secretId *string, approleId *string, outputMemCache bool) {
	agentToken := false
	if token != "" {
		agentToken = true
	}
	pwd, _ := os.Getwd()
	var content []byte

	if len(deployment) > 0 {
		config.DeploymentConfig = map[string]interface{}{"trcplugin": deployment}
		config.DeploymentCtlMessage = make(chan string, 5)
	}

	if config.EnvRaw == "itdev" {
		config.OutputMemCache = false
	}
	fmt.Println("Logging initialized.")
	config.Log.Printf("Logging initialized for env:%s\n", config.EnvRaw)

	// Chewbacca: scrub before checkin
	// This data is generated by TrcshAuth
	// cToken := ""
	// configRole := ""
	// pubRole := ""
	// fileBytes, err := ioutil.ReadFile("")
	// kc := base64.StdEncoding.EncodeToString(fileBytes)
	// gTrcshConfig = &trcshauth.TrcShConfig{Env: "dev",
	// 	EnvContext: "dev",
	// 	CToken:     &cToken,
	// 	ConfigRole: &configRole,
	// 	PubRole:    &pubRole,
	// 	KubeConfig: &kc,
	// }
	// config.VaultAddress = ""
	// config.Token = ""
	// Chewbacca: end scrub
	var err error
	config.Log.Printf("Bootstrap..")
	for {
		if gTrcshConfig == nil || gTrcshConfig.CToken == nil || gTrcshConfig.ConfigRole == nil || gTrcshConfig.VaultAddress == nil ||
			*gTrcshConfig.CToken == "" || *gTrcshConfig.ConfigRole == "" || *gTrcshConfig.VaultAddress == "" {
			gTrcshConfig, err = trcshauth.TrcshAuth(gAgentConfig, config)
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
	var auth string
	mergedVaultAddress := config.VaultAddress
	mergedEnvRaw := config.EnvRaw

	if (approleId != nil && len(*approleId) == 0) || (secretId != nil && len(*secretId) == 0) {
		// If in context of trcsh, utilize CToken to auth...
		if gTrcshConfig != nil && gTrcshConfig.CToken != nil {
			auth = *gTrcshConfig.CToken
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

	authTokenName := "vault_token_azuredeploy"
	authTokenEnv := "azuredeploy"
	autoErr := eUtils.AutoAuth(config, secretId, approleId, &auth, &authTokenName, &authTokenEnv, &mergedVaultAddress, &mergedEnvRaw, "deployauth", false)
	if autoErr != nil || auth == "" {
		fmt.Println("Unable to auth.")
		fmt.Println(autoErr)
		os.Exit(-1)
	}
	// End dbg comment
	if config.IsShell {
		config.Log.Println("Session Authorized")
	} else {
		fmt.Println("Session Authorized")
	}

	if (len(os.Args) > 1 || len(trcPath) > 0) && !strings.Contains(pwd, "TrcDeploy") {
		// Generate trc code...
		trcPathParts := strings.Split(trcPath, "/")
		config.FileFilter = []string{trcPathParts[len(trcPathParts)-1]}
		configRoleSlice := strings.Split(*gTrcshConfig.ConfigRole, ":")
		tokenName := "config_token_" + config.EnvRaw
		config.OutputMemCache = true
		config.StartDir = []string{"trc_templates"}
		config.EndDir = "."
		trcconfigbase.CommonMain(&config.EnvRaw, &mergedVaultAddress, &token, &mergedEnvRaw, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, nil, []string{"trcsh"}, config)
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
		if strings.Contains(pwd, "TrcDeploy") && len(config.DeploymentConfig) > 0 {
			if deployment, ok := config.DeploymentConfig["trcplugin"]; ok {
				// Swapping in project root...
				mod, err := helperkv.NewModifier(config.Insecure, *gTrcshConfig.CToken, *gTrcshConfig.VaultAddress, config.EnvRaw, config.Regions, true, config.Log)
				if err != nil {
					fmt.Println("Unable to obtain resources for deployment")
					return
				}
				mod.Env = config.EnvRaw
				deploymentConfig, err := mod.ReadData(fmt.Sprintf("super-secrets/Index/TrcVault/trcplugin/%s/Certify", deployment))
				if err != nil {
					fmt.Println("Unable to obtain config for deployment")
					return
				}
				config.DeploymentConfig = deploymentConfig
				if trcDeployRoot, ok := config.DeploymentConfig["trcdeployroot"]; ok {
					config.StartDir = []string{fmt.Sprintf("%s/trc_templates", trcDeployRoot.(string))}
					config.EndDir = trcDeployRoot.(string)
				}

				if trcProjectService, ok := config.DeploymentConfig["trcprojectservice"]; ok && strings.Contains(trcProjectService.(string), "/") {
					trcProjectServiceSlice := strings.Split(trcProjectService.(string), "/")
					config.ZeroConfig = true
					contentArray, _, _, err := vcutils.ConfigTemplate(config, mod, fmt.Sprintf("./trc_templates/%s/deploy/deploy.trc.tmpl", trcProjectService.(string)), true, trcProjectServiceSlice[0], trcProjectServiceSlice[1], false, true)
					config.ZeroConfig = false
					if err != nil {
						eUtils.LogErrorObject(config, err, false)
						return
					}
					content = []byte(contentArray)
				} else {
					fmt.Println("Project not configured and ready for deployment.  Missing projectservice")
					return
				}
			}
		} else {
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

			deployLine = strings.Trim(deployLine, " ")
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
			if utils.IsWindows() {
				// Log for traceability.
				config.Log.Println(deployLine)
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
					config.DeploymentCtlMessage <- fmt.Sprintf("%s\nEncountered errors: %s\n", deployLine, err.Error())
				} else {
					config.DeploymentCtlMessage <- deployLine
				}
			} else {
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
	if utils.IsWindows() {
		config.DeploymentCtlMessage <- capauth.TrcCtlComplete
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
