package main

import (
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
const envContextPrefix = "envContext:"

// This is a controller program that can act as any command line utility.
// The swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("Version: " + "1.34")
	envPtr := flag.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.
	envCtxPtr := flag.String("context", "", "Context to define.")

	var ctl string
	if !strings.HasPrefix(os.Args[1], "-") { //This pre checks arguments for ctl switch to allow parse to work with non "-" flags.
		ctl = os.Args[1]
		if len(os.Args) > 2 {
			os.Args = os.Args[1:]
		}
	}
	flag.Parse()

	dirname, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	//This will use env by default, if blank it will use context. If context is defined, it will replace context.
	if *envPtr == "" {
		file, err := ioutil.ReadFile(dirname + configDir)
		if err != nil {
			fmt.Printf("Could not read the context file due to this %s error \n", err)
			return
		}
		fileContent := string(file)

		if !strings.Contains(fileContent, envContextPrefix) && *envCtxPtr != "" {
			var output string
			if !strings.HasSuffix(fileContent, "\n") {
				output = fileContent + "\n" + envContextPrefix + *envCtxPtr
			} else {
				output = fileContent + envContextPrefix + *envCtxPtr
			}

			if err = ioutil.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
				fmt.Println(err.Error())
				return
			}
			*envPtr = *envCtxPtr
		} else {
			envContext := strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
			if *envCtxPtr != "" {
				output := strings.Replace(fileContent, envContextPrefix+envContext, envContextPrefix+*envCtxPtr, -1)
				if err = ioutil.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
					fmt.Println(err.Error())
					return
				}
				*envPtr = *envCtxPtr
			} else if *envPtr == "" {
				*envPtr = envContext
				*envCtxPtr = envContext
			}
		}
	} else {
		*envCtxPtr = *envPtr
		fmt.Println("Context flag will be ignored as env is defined.")
	}

	if ctl != "" {
		switch ctl {
		case "pub":
			trcpubbase.CommonMain(envPtr, nil, envCtxPtr)
		case "sub":
			trcsubbase.CommonMain(envPtr, nil, envCtxPtr)
		case "init":
			trcinitbase.CommonMain(envPtr, nil, envCtxPtr)
		case "config":
			trcconfigbase.CommonMain(envPtr, nil, envCtxPtr)
		case "x":
			trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, nil, nil, nil)
		}
	}
}
