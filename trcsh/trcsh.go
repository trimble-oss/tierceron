package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/dsnet/golib/memfile"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcconfigbase"
	"github.com/trimble-oss/tierceron/trcpubbase"
	kube "github.com/trimble-oss/tierceron/trcsh/kube/native"
	"github.com/trimble-oss/tierceron/trcsh/trcshauth"
	"github.com/trimble-oss/tierceron/trcvault/carrierfactory/capauth"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/utils/mlock"

	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

// This is a controller program that can act as any command line utility.
// The Tierceron Shell runs tierceron and kubectl commands in a secure shell.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("trcsh Version: " + "1.02")
	if os.Geteuid() == 0 {
		fmt.Println("Trcsh cannot be run as root.")
		os.Exit(-1)
	} else {
		capauth.CheckNotSudo()
	}
	envPtr := flag.String("env", "", "Environment to be seeded")      //If this is blank -> use context otherwise override context.
	trcPathPtr := flag.String("c", "", "Optional script to execute.") //If this is blank -> use context otherwise override context.

	flag.Parse()

	//Open deploy script and parse it.
	ProcessDeploy(*envPtr, "", *trcPathPtr)
}

func ProcessDeploy(env string, token string, trcPath string) {
	var err error
	agentToken := false
	if token != "" {
		agentToken = true
	}
	pwd, _ := os.Getwd()
	var content []byte
	if env == "" {
		env = os.Getenv("TRC_ENV")
	}
	fmt.Println("trcsh env: " + env)

	logFile := "./" + coreopts.GetFolderPrefix() + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.GetFolderPrefix()+"deploy.log" {
		logFile = "./" + coreopts.GetFolderPrefix() + "deploy.log"
	}
	f, _ := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	logger := log.New(f, "[DEPLOY]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: true,
		Log:            logger,
		IsShell:        true,
		OutputMemCache: true,
		MemCache:       map[string]*memfile.File{},
		ExitOnFailure:  true}

	if env == "itdev" {
		config.OutputMemCache = false
	}
	trcshConfig, err := trcshauth.TrcshAuth(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	if len(os.Args) > 1 || len(trcPath) > 0 {
		trcPathParts := strings.Split(trcPath, "/")
		config.FileFilter = []string{trcPathParts[len(trcPathParts)-1]}
		configRoleSlice := strings.Split(trcshConfig.ConfigRole, ":")
		tokenName := "config_token_" + env
		configEnv := env
		config.EnvRaw = env

		trcconfigbase.CommonMain(&configEnv, &config.VaultAddress, &token, &trcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, config)
		ResetModifier(config) //Resetting modifier cache to avoid token conflicts.

		if memCacheEntry, mcOk := config.MemCache[trcPath]; mcOk {
			content = memCacheEntry.Bytes()
		} else {
			fmt.Println("Error could not find " + os.Args[1] + " for deployment instructions")
		}
	} else {
		if env == "itdev" {
			content, err = ioutil.ReadFile(pwd + "/deploy/deploytest.trc")
			if err != nil {
				fmt.Println("Error could not find /deploy/deploytest.trc for deployment instructions")
			}
		} else {
			content, err = ioutil.ReadFile(pwd + "/deploy/deploy.trc")
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

	for _, deployLine := range deployArgLines {
		deployLine = strings.TrimLeft(deployLine, "")
		if strings.HasPrefix(deployLine, "#") {
			continue
		}
		fmt.Println(deployLine)
		os.Args = argsOrig
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.

		deployLine = strings.TrimRight(deployLine, "")
		deployArgs := strings.Split(deployLine, " ")
		control := deployArgs[0]
		if len(deployArgs) > 1 {
			envArgIndex := -1

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
				deployArgs = deployArgs[1:]
			}
			if control != "kubectl" {
				os.Args = append(os.Args, deployArgs...)
			}
		}

		switch control {
		case "trcpub":
			config.AppRoleConfig = "configpub.yml"
			pubRoleSlice := strings.Split(trcshConfig.PubRole, ":")
			tokenName := "vault_pub_token_" + env
			tokenPub := ""
			pubEnv := env
			config.EnvRaw = env

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
			configRoleSlice := strings.Split(trcshConfig.ConfigRole, ":")
			tokenName := "config_token_" + env
			tokenConfig := ""
			configEnv := env
			config.EnvRaw = env

			trcconfigbase.CommonMain(&configEnv, &config.VaultAddress, &tokenConfig, &trcshConfig.EnvContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, config)
			ResetModifier(config) //Resetting modifier cache to avoid token conflicts.

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

			// Placeholder.
			if deployArgs[0] == "config" {
				trcKubeDeploymentConfig.KubeContext = kube.ParseTrcKubeContext(trcKubeDeploymentConfig.KubeContext, deployArgs[1:])
			} else if deployArgs[0] == "create" {
				trcKubeDeploymentConfig.KubeDirective = kube.ParseTrcKubeDeployDirective(trcKubeDeploymentConfig.KubeDirective, deployArgs[1:])
				kube.CreateKubeResource(trcKubeDeploymentConfig, config)
			}

			fmt.Println(trcKubeDeploymentConfig.RestConfig.APIPath)
			fmt.Println(trcKubeDeploymentConfig.ApiConfig.APIVersion)
			//spew.Dump(kubeRestConfig)
			//spew.Dump(kubeApiConfig)

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
