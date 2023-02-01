package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"tierceron/buildopts/coreopts"
	"tierceron/trcconfigbase"
	"tierceron/trcpubbase"
	"tierceron/trcvault/opts/memonly"
	eUtils "tierceron/utils"
	"tierceron/utils/mlock"

	helperkv "tierceron/vaulthelper/kv"
)

const configDir = "/.tierceron/config.yml"
const envContextPrefix = "envContext: "

// This is a controller program that can act as any command line utility.
// The agent swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("trcagentctl Version: " + "1.00")
	envPtr := flag.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.

	flag.Parse()

	//Open deploy script and parse it.
	ProcessDeploy(*envPtr, "")
}

func ProcessDeploy(env string, token string) {
	var err error
	agentToken := false
	if token != "" {
		agentToken = true
	}
	pwd, _ := os.Getwd()
	content, err := ioutil.ReadFile(pwd + "/deploy/deploy.trc")
	if err != nil {
		fmt.Println("Error could not find /deploy/deploy.trc for deployment instructions")
	}

	deployArgLines := strings.Split(string(content), "\n")
	configCount := strings.Count(string(content), "trcconfig") //Uses this to close result channel on last run.

	logFile := "./" + coreopts.GetFolderPrefix() + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.GetFolderPrefix()+"deploy.log" {
		logFile = "./" + coreopts.GetFolderPrefix() + "deploy.log"
	}
	f, _ := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[DEPLOY]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}

	var addrPort string
	var envContext string
	//Env should come from command line - not context here. but addr port is needed.
	env, envContext, addrPort, err = GetSetEnvAddrContext(env, envContext, addrPort)
	if err != nil {
		fmt.Println(err)
		return
	}
	addr := "https://127.0.0.1:" + addrPort
	config.VaultAddress = addr
	config.Env = env
	config.EnvRaw = env

	for _, deployLine := range deployArgLines {
		fmt.Println(deployLine)
		deployLine = strings.TrimPrefix(deployLine, "trc")
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
			os.Args = append(os.Args, deployArgs...)
		}

		switch control {
		case "pub":
			config.FileFilter = nil
			config.FileFilter = append(config.FileFilter, "configpub.yml")
			trcpubbase.CommonMain(&env, &addr, &token, &envContext, config)
			ResetModifier(config)                                            //Resetting modifier cache to avoid token conflicts.
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.
			env = *flag.String("env", config.Env, "Environment to be seeded")
			if !agentToken {
				token = ""
				config.Token = token
			}
		case "config":
			configCount -= 1
			if configCount != 0 { //This is to keep result channel open - closes on the final config call of the script.
				config.EndDir = "deploy"
			}
			config.FileFilter = nil
			config.FileFilter = append(config.FileFilter, "config.yml")
			trcconfigbase.CommonMain(&env, &addr, &token, &envContext, config)
			ResetModifier(config)                                            //Resetting modifier cache to avoid token conflicts.
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) //Reset flag parse to allow more toolset calls.
			env = *flag.String("env", config.Env, "Environment to be seeded")
			if !agentToken {
				token = ""
				config.Token = token
			}
		case "kubectl":

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

func GetSetEnvAddrContext(env string, envContext string, addrPort string) (string, string, string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}

	//This will use env by default, if blank it will use context. If context is defined, it will replace context.
	if env == "" {
		file, err := ioutil.ReadFile(dirname + configDir)
		if err != nil {
			fmt.Printf("Could not read the context file due to this %s error \n", err)
			return "", "", "", err
		}
		fileContent := string(file)
		if fileContent == "" {
			return "", "", "", errors.New("Could not read the context file")
		}
		if !strings.Contains(fileContent, envContextPrefix) && envContext != "" {
			var output string
			if !strings.HasSuffix(fileContent, "\n") {
				output = fileContent + "\n" + envContextPrefix + envContext + "\n"
			} else {
				output = fileContent + envContextPrefix + envContext + "\n"
			}

			if err = ioutil.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
				return "", "", "", err
			}
			fmt.Println("Context flag has been written out.")
			env = envContext
		} else {
			re := regexp.MustCompile(`[-]?\d[\d,]*[\.]?[\d{2}]*`)
			result := re.FindAllString(fileContent[:strings.Index(fileContent, "\n")], -1)
			if len(result) == 1 {
				addrPort = result[0]
			} else {
				return "", "", "", errors.New("Couldn't find port.")
			}
			currentEnvContext := strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
			if envContext != "" {
				output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
				if err = ioutil.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
					return "", "", "", err
				}
				fmt.Println("Context flag has been written out.")
				env = envContext
			} else if env == "" {
				env = currentEnvContext
				envContext = currentEnvContext
			}
		}
	} else {
		envContext = env
		fmt.Println("Context flag will be ignored as env is defined.")
	}
	return env, envContext, addrPort, nil
}
