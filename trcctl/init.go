package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"tierceron/trcconfigbase"
	trcinitbase "tierceron/trcinitbase"
	"tierceron/trcpubbase"
	"tierceron/trcsubbase"
	"tierceron/trcvault/opts/memonly"
	"tierceron/trcx/xutil"
	"tierceron/trcxbase"
	"tierceron/utils/mlock"
)

const configDir = "/.tierceron/config.yml"
const envContextPrefix = "envContext: "

// This is a controller program that can act as any command line utility.
// The swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.34")
	envPtr := flag.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.
	var envContext string

	var ctl string
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") { //This pre checks arguments for ctl switch to allow parse to work with non "-" flags.
		ctl = os.Args[1]
		ctlSplit := strings.Split(ctl, " ")
		if len(ctlSplit) >= 2 {
			fmt.Println("Invalid arguments - only 1 non flag argument available at a time.")
			return
		}

		if len(os.Args) > 2 {
			os.Args = os.Args[1:]
		}
	}
	flag.Parse()

	if ctl != "" {
		var err error
		if strings.Contains(ctl, "context") {
			contextSplit := strings.Split(ctl, "=")
			if len(contextSplit) == 1 {
				*envPtr, envContext, err = GetSetEnvContext(*envPtr, envContext)
				if err != nil {
					fmt.Println(err.Error())
					return
				}
				fmt.Println("Current context is set to " + envContext)
			} else if len(contextSplit) == 2 {
				envContext = contextSplit[1]
				*envPtr, envContext, err = GetSetEnvContext(*envPtr, envContext)
				if err != nil {
					fmt.Println(err.Error())
					return
				}
			}
		}

		*envPtr, envContext, err = GetSetEnvContext(*envPtr, envContext)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		if len(os.Args) == 2 && ctl != "" {
			os.Args = os.Args[0:1]
		}
		switch ctl {
		case "pub":
			trcpubbase.CommonMain(envPtr, nil, &envContext)
		case "sub":
			trcsubbase.CommonMain(envPtr, nil, &envContext)
		case "init":
			trcinitbase.CommonMain(envPtr, nil, &envContext)
		case "config":
			trcconfigbase.CommonMain(envPtr, nil, &envContext)
		case "x":
			trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, nil, nil, nil)
		}
	}
}

func GetSetEnvContext(env string, envContext string) (string, string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	//This will use env by default, if blank it will use context. If context is defined, it will replace context.
	if env == "" {
		file, err := ioutil.ReadFile(dirname + configDir)
		if err != nil {
			fmt.Printf("Could not read the context file due to this %s error \n", err)
			return "", "", err
		}
		fileContent := string(file)
		if fileContent == "" {
			return "", "", errors.New("Could not read the context file")
		}
		if !strings.Contains(fileContent, envContextPrefix) && envContext != "" {
			var output string
			if !strings.HasSuffix(fileContent, "\n") {
				output = fileContent + "\n" + envContextPrefix + envContext + "\n"
			} else {
				output = fileContent + envContextPrefix + envContext + "\n"
			}

			if err = ioutil.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
				return "", "", err
			}
			fmt.Println("Context flag has been written out.")
			env = envContext
		} else {
			currentEnvContext := strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
			if envContext != "" {
				output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
				if err = ioutil.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
					return "", "", err
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
	return env, envContext, nil
}
