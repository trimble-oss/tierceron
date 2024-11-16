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
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase"
	trcinitbase "github.com/trimble-oss/tierceron/pkg/cli/trcinitbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcpubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcsubbase"
	"github.com/trimble-oss/tierceron/pkg/cli/trcxbase"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
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
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	fmt.Println("Version: " + "1.36")
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "", "Environment to be seeded") //If this is blank -> use context otherwise override context.
	pluginNamePtr := flagset.String("pluginName", "", "Specifies which templates to filter")
	tokenPtr := flagset.String("token", "", "Vault access token")
	uploadCertPtr := flagset.Bool("certs", false, "Upload certs if provided")
	flagset.Bool("prod", false, "Prod only seeds vault with staging environment")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
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

		switch ctl {
		case "pub":
			tokenName := fmt.Sprintf("vault_pub_token_%s", eUtils.GetEnvBasis(*envPtr))
			driverConfig := config.DriverConfig{
				CoreConfig: &core.CoreConfig{
					TokenCache:    cache.NewTokenCacheEmpty(),
					ExitOnFailure: true,
				},
			}
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			trcpubbase.CommonMain(envPtr, addrPtr, &envContext, nil, nil, &tokenName, flagset, os.Args, &driverConfig)
		case "sub":
			tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
			driverConfig := config.DriverConfig{
				CoreConfig: &core.CoreConfig{
					TokenCache:    cache.NewTokenCacheEmpty(),
					ExitOnFailure: true,
				},
			}
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			trcsubbase.CommonMain(envPtr, addrPtr, &envContext, nil, nil, &tokenName, flagset, os.Args, &driverConfig)
		case "init":
			//tokenName := fmt.Sprintf("config_token_%s_unrestricted", eUtils.GetEnvBasis(*envPtr))
			driverConfig := config.DriverConfig{
				CoreConfig: &core.CoreConfig{
					TokenCache:    cache.NewTokenCacheEmpty(),
					ExitOnFailure: true,
				},
			}
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			flagset.String("env", "dev", "Environment to configure")
			flagset.String("addr", "", "API endpoint for the vault")
			trcinitbase.CommonMain(envPtr, addrPtr, &envContext, nil, nil, nil, uploadCertPtr, flagset, os.Args, &driverConfig)
		case "config":
			tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
			driverConfig := config.DriverConfig{
				CoreConfig: &core.CoreConfig{
					TokenCache:    cache.NewTokenCacheEmpty(),
					ExitOnFailure: true,
				},
			}
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			trcconfigbase.CommonMain(envPtr, addrPtr, &envContext, nil, nil, &tokenName, nil, nil, os.Args, &driverConfig)
		case "subx":
			tokenName := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(*envPtr))
			driverConfig := config.DriverConfig{
				CoreConfig: &core.CoreConfig{
					TokenCache:    cache.NewTokenCache(tokenName, tokenPtr),
					ExitOnFailure: true,
				},
			}
			if len(*pluginNamePtr) == 0 {
				fmt.Printf("Must specify either -pluginName flag \n")
				return
			}

			os.Mkdir(*pluginNamePtr, 0700)
			os.Chdir(*pluginNamePtr)
			fmt.Printf("%s\n", *pluginNamePtr)
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			fmt.Printf("%v\n", os.Args)
			retrictedMappingsMap := coreopts.BuildOptions.GetPluginRestrictedMappings()

			if pluginRestrictedMappings, ok := retrictedMappingsMap[*pluginNamePtr]; ok {

				os.Mkdir("trc_seeds", 0700)
				for _, restrictedMapping := range pluginRestrictedMappings {
					restrictedMappingSub := append([]string{"", os.Args[1]}, restrictedMapping[0])
					flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
					trcsubbase.CommonMain(envPtr, addrPtr, &envContext, nil, nil, &tokenName, flagset, restrictedMappingSub, &driverConfig)
					restrictedMappingX := append([]string{""}, restrictedMapping[1:]...)
					if eUtils.RefLength(tokenPtr) > 0 {
						restrictedMappingX = append(restrictedMappingX, fmt.Sprintf("-tokenName=%s", tokenName))
						restrictedMappingX = append(restrictedMappingX, fmt.Sprintf("-token=%s", *tokenPtr))
					}
					flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
					trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, addrPtr, &envContext, nil, flagset, restrictedMappingX)
				}
			}
			os.Chdir("..")
		case "x":
			flagset = flag.NewFlagSet(ctl, flag.ExitOnError)
			trcxbase.CommonMain(nil, xutil.GenerateSeedsFromVault, envPtr, addrPtr, &envContext, nil, flagset, os.Args)
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

			if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
				return "", "", err
			}
			fmt.Println("Context flag has been written out.")
			env = envContext
		} else {
			currentEnvContext := "dev"
			if strings.Index(fileContent, envContextPrefix) > 0 {
				currentEnvContext = strings.TrimSpace(fileContent[strings.Index(fileContent, envContextPrefix)+len(envContextPrefix):])
			}
			if envContext != "" {
				output := strings.Replace(fileContent, envContextPrefix+currentEnvContext, envContextPrefix+envContext, -1)
				if err = os.WriteFile(dirname+configDir, []byte(output), 0600); err != nil {
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
