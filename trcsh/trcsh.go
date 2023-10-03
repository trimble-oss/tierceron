package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcconfigbase"
	"github.com/trimble-oss/tierceron/trcpubbase"
	kube "github.com/trimble-oss/tierceron/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/trcsh/trcshauth"
	"github.com/trimble-oss/tierceron/trcvault/carrierfactory/capauth"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"

	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	eUtils.InitHeadless(true)
	fmt.Println("trcsh Version: " + "1.11")

	if os.Geteuid() == 0 {
		fmt.Println("Trcsh cannot be run as root.")
		os.Exit(-1)
	} else {
		capauth.CheckNotSudo()
	}
	if len(os.Args) > 1 {
		if strings.Contains(os.Args[1], "trc") && !strings.Contains(os.Args[1], "-c") {
			// Running as shell.
			os.Args[1] = "-c=" + os.Args[1]
		}
	}
	envPtr := flag.String("env", "", "Environment to be processed")   //If this is blank -> use context otherwise override context.
	regionPtr := flag.String("region", "", "Region to be processed")  //If this is blank -> use context otherwise override context.
	trcPathPtr := flag.String("c", "", "Optional script to execute.") //If this is blank -> use context otherwise override context.
	appRoleIDPtr := flag.String("appRoleID", "", "Public app role ID")
	secretIDPtr := flag.String("secretID", "", "App role secret")

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
	ProcessDeploy(*envPtr, *regionPtr, "", *trcPathPtr, secretIDPtr, appRoleIDPtr)
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
func ProcessDeploy(env string, region string, token string, trcPath string, secretId *string, approleId *string) {
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
		Log:               logger,
		IsShell:           true,
		IsShellSubProcess: false,
		OutputMemCache:    true,
		MemFs:             memfs.New(),
		Regions:           regions,
		ExitOnFailure:     true}

	if env == "itdev" {
		config.OutputMemCache = false
	}
	fmt.Println("Logging initialized.")
	logger.Printf("Logging initialized for env:%s\n", env)

	// 	trcshConfig := &trcshauth.TrcShConfig{Env: "",
	// 		EnvContext: "",
	// 		ConfigRole: "",
	// 		PubRole:    "",
	// 	}
	trcshConfig, err := trcshauth.TrcshAuth(config)
	if err != nil {
		fmt.Println("Tierceron bootstrap failure.")
		logger.Println(err)
		os.Exit(-1)
	}
	fmt.Println("Auth loaded" + env)

	// Begin dbg comment
	var auth string
	authTokenName := "vault_token_azuredeploy"
	authTokenEnv := "azuredeploy"
	autoErr := eUtils.AutoAuth(config, secretId, approleId, &auth, &authTokenName, &authTokenEnv, &config.VaultAddress, &config.EnvRaw, "deployauth", false)
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
		configRoleSlice := strings.Split(trcshConfig.ConfigRole, ":")
		tokenName := "config_token_" + env
		configEnv := env
		config.EnvRaw = env
		config.EndDir = "deploy"
		config.OutputMemCache = true
		trcconfigbase.CommonMain(&configEnv, &config.VaultAddress, &token, &trcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, &region, config)
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
		if strings.HasPrefix(deployPipeline, "#") {
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

			switch control {
			case "trcpub":
				config.AppRoleConfig = "configpub.yml"
				config.EnvRaw = env
				config.IsShellSubProcess = true

				pubRoleSlice := strings.Split(trcshConfig.PubRole, ":")
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
				configCount -= 1
				if configCount != 0 { //This is to keep result channel open - closes on the final config call of the script.
					config.EndDir = "deploy"
				}
				config.AppRoleConfig = "config.yml"
				config.FileFilter = nil
				config.EnvRaw = env
				config.WantCerts = false
				config.IsShellSubProcess = true

				configRoleSlice := strings.Split(trcshConfig.ConfigRole, ":")
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
			case "trcpluginctl":
			case "kubectl":
				onceKubeInit.Do(func() {
					var kubeInitErr error
					trcKubeDeploymentConfig, kubeInitErr = kube.InitTrcKubeConfig(trcshConfig, config)
					if kubeInitErr != nil {
						fmt.Println(kubeInitErr)
						return
					}
				})
				trcKubeDeploymentConfig.PipeOS = PipeOS

				kubectlErrChan := make(chan error, 1)

				go func() {
					kubectlErrChan <- kube.KubeCtl(trcKubeDeploymentConfig, config)
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
