package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	trcinitbase "github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcxbase"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"
)

const configDir = "/.tierceron/config.yml"
const envContextPrefix = "envContext: "

// This is a controller program that can act as any command line utility.
// The swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	fmt.Println("Version: " + "1.36")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.
	tokenPtr := flagset.String("token", "", "Vault access token")
	secretIDPtr := flagset.String("secretID", "", "Secret app role ID")
	appRoleIDPtr := flagset.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config to access the vault")
	flagset.Bool("diff", false, "Diff files")
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
	flagset.Parse(os.Args[1:])
	if flagset.NFlag() == 0 {
		flagset.Usage()
		os.Exit(0)
	}

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
		var addrPtr string
		switch ctl {
		case "pub":
			trcpubbase.CommonMain(envPtr, &addrPtr, tokenPtr, &envContext, secretIDPtr, appRoleIDPtr, tokenNamePtr, flagset, os.Args, nil)
		case "sub":
			trcsubbase.CommonMain(envPtr, &addrPtr, &envContext, secretIDPtr, appRoleIDPtr, flagset, os.Args, nil)
		case "init":
			trcinitbase.CommonMain(envPtr, &addrPtr, &envContext, flagset, os.Args)
		case "config":
			trcconfigbase.CommonMain(envPtr, &addrPtr, tokenPtr, &envContext, secretIDPtr, appRoleIDPtr, tokenNamePtr, nil, nil, os.Args, nil)
		case "x":
			trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, &addrPtr, &envContext, nil, nil, os.Args)
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
		file, err := os.ReadFile(dirname + configDir)
		if err != nil {
			fmt.Printf("Could not read the context file due to this %s error \n", err)
			return "", "", err
		}
		fileContent := string(file)
		if fileContent == "" {
			return "", "", errors.New("could not read the context file")
		}
		if !strings.Contains(fileContent, envContextPrefix) && envContext != "" {
			var output string
			if !strings.HasSuffix(fileContent, "\n") {
				output = fileContent + "\n" + envContextPrefix + envContext + "\n"
			} else {
				output = fileContent + envContextPrefix + envContext + "\n"
			}

			if err = os.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
				return "", "", err
			}
			fmt.Println("Context flag has been written out.")
			env = envContext
		} else {
			currentEnvContext := strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
			if envContext != "" {
				output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
				if err = os.WriteFile(dirname+configDir, []byte(output), 0666); err != nil {
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
