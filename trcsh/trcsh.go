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
	"runtime"
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
	"github.com/trimble-oss/tierceron/trcpubbase"
	kube "github.com/trimble-oss/tierceron/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/trcsh/trcshauth"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	"github.com/trimble-oss/tierceron/trcvault/trcplgtoolbase"
	eUtils "github.com/trimble-oss/tierceron/utils"

	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

var gAgentConfig *capauth.AgentConfigs = nil
var gTrcshConfig *capauth.TrcShConfig

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	eUtils.InitHeadless(true)
	fmt.Println("trcsh Version: " + "1.20")
	var envPtr, regionPtr, trcPathPtr, appRoleIDPtr, secretIDPtr *string

	if runtime.GOOS != "windows" {
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
				var ic chan os.Signal = make(chan os.Signal)
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

		//Open deploy script and parse it.
		ProcessDeploy(*envPtr, *regionPtr, "", *trcPathPtr, secretIDPtr, appRoleIDPtr, true)
	} else {
		gAgentConfig = &capauth.AgentConfigs{CtlMessage: make(chan string, 1)}
		deployments := os.Getenv("DEPLOYMENTS")
		agentToken := os.Getenv("AGENT_TOKEN")
		agentEnv := os.Getenv("AGENT_ENV")
		address := os.Getenv("VAULT_ADDR")

		envPtr = flag.String("env", "", "Environment to be processed")   //If this is blank -> use context otherwise override context.
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
		for {
			if deployFlapMode, deployEmitErr := cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
				*gAgentConfig.EncryptSalt,
				*gAgentConfig.HandshakeHostPort,
				*gAgentConfig.HandshakeCode,
				cap.MODE_FLAP, deployments+"."+*gAgentConfig.Env); deployEmitErr == nil && strings.HasPrefix(deployFlapMode, cap.MODE_GAZE) {
				go func() {
					for {
					perching:
						select {
						case <-time.After(120 * time.Second):
							ctlMsg := "Deployment timed out after 120 seconds"
							cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
								*gAgentConfig.EncryptSalt,
								*gAgentConfig.HandshakeHostPort,
								*gAgentConfig.HandshakeCode,
								cap.MODE_PERCH+"_"+ctlMsg, deployments+"."+*gAgentConfig.Env)
							gAgentConfig.CtlMessage <- capauth.TrcCtlComplete
						case ctlMsg := <-gAgentConfig.CtlMessage:
							if ctlMsg != capauth.TrcCtlComplete {
								deployLineFlapMode := cap.MODE_FLAP + "_" + ctlMsg
								ctlFlapMode := deployLineFlapMode
								var err error
								for {
									if err == nil && ctlFlapMode == cap.MODE_PERCH {
										// Acknowledge perching...
										cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
											*gAgentConfig.EncryptSalt,
											*gAgentConfig.HandshakeHostPort,
											*gAgentConfig.HandshakeCode,
											cap.MODE_PERCH, deployments+"."+*gAgentConfig.Env)
										ctlFlapMode = cap.MODE_PERCH
										goto perching
									}

									// Notify deployer of command run and wait for confirmation of msg received.
									if err == nil && deployLineFlapMode != ctlFlapMode {
										// Flap, Gaze, etc...
										break
									} else {
										callFlap := deployLineFlapMode
										if err == nil {
											time.Sleep(200 * time.Millisecond)
										} else {
											if err.Error() != "init" {
												time.Sleep(1 * time.Second)
											}
										}
										deployLineFlapMode, err = cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
											*gAgentConfig.EncryptSalt,
											*gAgentConfig.HandshakeHostPort,
											*gAgentConfig.HandshakeCode,
											callFlap, deployments+"."+*gAgentConfig.Env)
									}
								}
							} else {
								cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
									*gAgentConfig.EncryptSalt,
									*gAgentConfig.HandshakeHostPort,
									*gAgentConfig.HandshakeCode,
									cap.MODE_GLIDE, deployments+"."+*gAgentConfig.Env)
							}
						}
					}
				}()
				ProcessDeploy(*gAgentConfig.Env, *regionPtr, "", *trcPathPtr, secretIDPtr, appRoleIDPtr, false)

			} else {
				time.Sleep(500 * time.Millisecond)
			}
		}
		<-shutdown
	}

}

var interruptChan chan os.Signal = make(chan os.Signal)
var twoHundredMilliInterruptTicker *time.Ticker = time.NewTicker(200 * time.Millisecond)
var multiSecondInterruptTicker *time.Ticker = time.NewTicker(time.Second)

func interruptFun(tickerInterrupt *time.Ticker) {
	select {
	case <-interruptChan:
		cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
			*gAgentConfig.EncryptSalt,
			*gAgentConfig.HandshakeHostPort,
			*gAgentConfig.HandshakeCode,
			cap.MODE_PERCH, *gAgentConfig.Deployments+"."+*gAgentConfig.Env)
		os.Exit(1)
	case <-tickerInterrupt.C:
	}
}

func featherCtlCb(agentName string) error {

	if gAgentConfig == nil {
		return errors.New("Incorrect agent initialization")
	} else {
		gAgentConfig.Deployments = &agentName
	}
	callFlap := cap.MODE_GAZE
	for {
		// Azure deployment agent kicks off a deploy with a flap command...
		if ctlFlapMode, featherErr := cap.FeatherCtlEmit(*gAgentConfig.EncryptPass,
			*gAgentConfig.EncryptSalt,
			*gAgentConfig.HandshakeHostPort,
			*gAgentConfig.HandshakeCode,
			callFlap, agentName+"."+*gAgentConfig.Env); featherErr != nil || ctlFlapMode == cap.MODE_PERCH || ctlFlapMode == cap.MODE_GLIDE {
			if featherErr != nil {
				fmt.Printf("\nDeployment error.\n")
			} else {
				fmt.Printf("\nDeployment complete.\n")
			}
			os.Exit(0)
		} else {
			if strings.HasPrefix(ctlFlapMode, cap.MODE_FLAP) {
				if strings.Contains(ctlFlapMode, "_") {
					ctlMessage := strings.Split(ctlFlapMode, "_")
					if len(ctlMessage) > 1 {
						fmt.Printf("%s\n", ctlMessage[1])
					}
				}
			}
			callFlap = cap.MODE_GAZE
			interruptFun(multiSecondInterruptTicker)
		}
	}

}

func configCmd(env string,
	trcshConfig *capauth.TrcShConfig,
	region string,
	config *eUtils.DriverConfig,
	agentToken bool,
	token string,
	argsOrig []string,
	deployArgLines []string,
	configCount *int) {
	*configCount -= 1
	if *configCount != 0 { //This is to keep result channel open - closes on the final config call of the script.
		config.EndDir = "deploy"
	}
	config.AppRoleConfig = "config.yml"
	config.FileFilter = nil
	config.EnvRaw = env
	config.WantCerts = false
	config.IsShellSubProcess = true

	configRoleSlice := strings.Split(*trcshConfig.ConfigRole, ":")
	tokenName := "config_token_" + env
	tokenConfig := ""
	configEnv := env

	trcconfigbase.CommonMain(&configEnv, &config.VaultAddress, &tokenConfig, &trcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, config)
	ResetModifier(config)                                            //Resetting modifier cache to avoid token conflicts.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.

	if !agentToken {
		token = ""
		config.Token = token
	}
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
	configCount *int,
	logger *log.Logger) {
	switch control {
	case "trcpub":
		config.AppRoleConfig = "configpub.yml"
		config.EnvRaw = env
		config.IsShellSubProcess = true

		pubRoleSlice := strings.Split(*gTrcshConfig.PubRole, ":")
		tokenName := "vault_pub_token_" + env
		tokenPub := ""
		pubEnv := env

		trcpubbase.CommonMain(&pubEnv, &config.VaultAddress, &tokenPub, &trcshConfig.EnvContext, &pubRoleSlice[1], &pubRoleSlice[0], &tokenName, config)
		ResetModifier(config)                                            //Resetting modifier cache to avoid token conflicts.
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.
		if !agentToken {
			token = ""
			config.Token = token
		}
	case "trcconfig":
		configCmd(env, trcshConfig, region, config, agentToken, token, argsOrig, deployArgLines, configCount)
	case "trcplgtool":
		config.AppRoleConfig = ""
		config.EnvRaw = env
		config.IsShellSubProcess = true

		config.FeatherCtlCb = featherCtlCb
		if gAgentConfig == nil {
			gAgentConfig = &capauth.AgentConfigs{CtlMessage: make(chan string, 1)}
			gAgentConfig.LoadConfigs(config.VaultAddress, *trcshConfig.CToken, "bootstrap", "dev") // Feathering always in dev environmnent.
		}
		gAgentConfig.Env = &env

		trcplgtoolbase.CommonMain(&env, &config.VaultAddress, trcshConfig.CToken, &region, config)
		config.FeatherCtlCb = nil
		ResetModifier(config)                                            //Resetting modifier cache to avoid token conflicts.
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.
		if !agentToken {
			token = ""
			config.Token = token
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

		go func() {
			config.Log.Println("Executing kubectl")
			kubectlErrChan <- kube.KubeCtl(*trcKubeDeploymentConfig, config)
		}()

		select {
		case <-time.After(15 * time.Second):
			fmt.Println("Agent is not yet ready..")
			logger.Println("Timed out waiting for KubeCtl.")
			os.Exit(-1)
		case kubeErr := <-kubectlErrChan:
			if kubeErr != nil {
				logger.Println(kubeErr)
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
	configCount *int,
	logger *log.Logger) {
	switch control {
	case "trcplgtool":
		config.AppRoleConfig = ""
		config.EnvRaw = env
		config.IsShellSubProcess = true

		trcplgtoolbase.CommonMain(&env, &config.VaultAddress, trcshConfig.CToken, &region, config)
		ResetModifier(config)
		os.Args = []string{os.Args[0]}                                   //Resetting modifier cache to avoid token conflicts.
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.
		if !agentToken {
			token = ""
			config.Token = token
		}
	case "trcconfig":
		configCmd(env, trcshConfig, region, config, agentToken, token, argsOrig, deployArgLines, configCount)
		ResetModifier(config)                                            //Resetting modifier cache to avoid token conflicts.
		os.Args = []string{os.Args[0]}                                   //Resetting modifier cache to avoid token conflicts.
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.
		if !agentToken {
			token = ""
			config.Token = token
		}
	}
}

// ProcessDeploy
//
// Parameters:
//
//   - env: Current environment context
//   - token: An environment token
//   - trcPath: Path to the current executable
//   - secretId: trcsh secret.
//   - approleId: trcsh app role.
//
// Returns:
//
//	Nothing.
func ProcessDeploy(env string, region string, token string, trcPath string, secretId *string, approleId *string, outputMemCache bool) {
	var err error
	agentToken := false
	if token != "" {
		agentToken = true
	}
	pwd, _ := os.Getwd()
	var content []byte
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
				os.Exit(1)
			}
		}
	}

	fmt.Println("trcsh env: " + env)
	fmt.Printf("trcsh regions: %s\n", strings.Join(regions, ", "))

	logFile := "./" + coreopts.GetFolderPrefix(nil) + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.GetFolderPrefix(nil)+"deploy.log" {
		logFile = "./" + coreopts.GetFolderPrefix(nil) + "deploy.log"
	}
	f, _ := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	logger := log.New(f, "[DEPLOY]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: true,
		EnvRaw:            env,
		Log:               logger,
		IsShell:           true,
		IsShellSubProcess: false,
		OutputMemCache:    outputMemCache,
		MemFs:             memfs.New(),
		Regions:           regions,
		ExitOnFailure:     true}

	if env == "itdev" {
		config.OutputMemCache = false
	}
	fmt.Println("Logging initialized.")
	logger.Printf("Logging initialized for env:%s\n", env)

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
	if gTrcshConfig == nil {
		gTrcshConfig, err = trcshauth.TrcshAuth(gAgentConfig, config)
		if err != nil {
			fmt.Println("Tierceron bootstrap failure.")
			fmt.Println(err.Error())
			logger.Println(err)
			os.Exit(-1)
		}
		fmt.Printf("Auth loaded %s\n", env)
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
	fmt.Println("Session Authorized")
	if len(os.Args) > 1 || len(trcPath) > 0 {
		// Generate .trc code...
		trcPathParts := strings.Split(trcPath, "/")
		config.FileFilter = []string{trcPathParts[len(trcPathParts)-1]}
		configRoleSlice := strings.Split(*gTrcshConfig.ConfigRole, ":")
		tokenName := "config_token_" + env
		configEnv := env
		config.EnvRaw = env
		config.EndDir = "deploy"
		config.OutputMemCache = true
		trcconfigbase.CommonMain(&configEnv, &mergedVaultAddress, &token, &mergedEnvRaw, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, config)
		ResetModifier(config) //Resetting modifier cache to avoid token conflicts.

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
		if env == "itdev" || env == "staging" || env == "prod" {
			config.OutputMemCache = false
		}
		os.Args = []string{os.Args[0]}
		fmt.Println("Processing trcshell")

	} else {
		fmt.Println("Processing manual trcshell")
		if env == "itdev" {
			content, err = os.ReadFile(pwd + "/deploy/buildtest.trc")
			if err != nil {
				fmt.Println("Error could not find /deploy/buildtest.trc for deployment instructions")
			}
		} else {
			content, err = os.ReadFile(pwd + "/deploy/deploy.trc")
			if err != nil {
				fmt.Println("Error could not find " + pwd + " /deploy/deploy.trc for deployment instructions")
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
			if runtime.GOOS == "windows" {
				processWindowsCmds(
					trcKubeDeploymentConfig,
					&onceKubeInit,
					PipeOS,
					env,
					gTrcshConfig,
					region,
					config,
					control,
					agentToken,
					token,
					argsOrig,
					deployArgLines,
					&configCount,
					logger)
				gAgentConfig.CtlMessage <- deployLine
			} else {
				processPluginCmds(
					&trcKubeDeploymentConfig,
					&onceKubeInit,
					PipeOS,
					env,
					gTrcshConfig,
					region,
					config,
					control,
					agentToken,
					token,
					argsOrig,
					deployArgLines,
					&configCount,
					logger)
			}
		}
	}
	if runtime.GOOS == "windows" {
		gAgentConfig.CtlMessage <- capauth.TrcCtlComplete
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
