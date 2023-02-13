package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/dsnet/golib/memfile"
	"github.com/trimble-oss/tierceron-hat/cap"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/trcconfigbase"
	"github.com/trimble-oss/tierceron/trcpubbase"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/utils/mlock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

const configDir = "/.tierceron/config.yml"
const envContextPrefix = "envContext: "

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func penseQuery(pense string) (string, error) {
	penseCode := randomString(7 + rand.Intn(7))
	penseArray := sha256.Sum256([]byte(penseCode))
	penseSum := hex.EncodeToString(penseArray[:])

	capWritErr := cap.TapWriter(penseSum)
	if capWritErr != nil {
		return "", capWritErr
	}

	conn, err := grpc.Dial("127.0.0.1:12384", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()
	c := cap.NewCapClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.Pense(ctx, &cap.PenseRequest{Pense: penseCode, PenseIndex: pense})
	if err != nil {
		return "", err
	}

	return r.GetPense(), nil
}

// This is a controller program that can act as any command line utility.
// The agent swiss army knife of tierceron if you will.
func main() {
	if memonly.IsMemonly() {
		mlock.Mlock(nil)
	}
	fmt.Println("trcagentctl Version: " + "1.01")
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
	var content []byte
	if env == "itdev" {
		content, err = ioutil.ReadFile(pwd + "/deploy/deploytest.trc")
		if err != nil {
			fmt.Println("Error could not find /deploy/deploytest.trc for deployment instructions")
		}
	} else {
		content, err = ioutil.ReadFile(pwd + "/deploy/deploy.trc")
		if err != nil {
			fmt.Println("Error could not find /deploy/deploy.trc for deployment instructions")
		}
	}

	deployArgLines := strings.Split(string(content), "\n")
	configCount := strings.Count(string(content), "trcconfig") //Uses this to close result channel on last run.

	logFile := "./" + coreopts.GetFolderPrefix() + "deploy.log"
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && logFile == "/var/log/"+coreopts.GetFolderPrefix()+"deploy.log" {
		logFile = "./" + coreopts.GetFolderPrefix() + "deploy.log"
	}
	f, _ := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[DEPLOY]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: true,
		Log:            logger,
		OutputMemCache: true,
		MemCache:       map[string]*memfile.File{},
		ExitOnFailure:  true}

	if env == "itdev" {
		config.OutputMemCache = false
	}

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
			os.Args = append(os.Args, deployArgs...)
		}

		switch control {
		case "pub":
			config.FileFilter = nil
			config.FileFilter = append(config.FileFilter, "configpub.yml")
			pubRole, penseErr := penseQuery("pubrole")
			if penseErr != nil {
				fmt.Println(err)
				return
			}
			pubRoleSlice := strings.Split(pubRole, ":")
			tokenName := "pub_token_" + env

			trcpubbase.CommonMain(&env, &addr, &token, &envContext, &pubRoleSlice[0], &pubRoleSlice[1], &tokenName, config)
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
			addr, vAddressErr := penseQuery("vaddress")
			if vAddressErr != nil {
				fmt.Println(vAddressErr)
				return
			}
			configRole, penseErr := penseQuery("configrole")
			if penseErr != nil {
				fmt.Println(penseErr)
				return
			}
			configRoleSlice := strings.Split(configRole, ":")
			tokenName := "config_token_" + env

			trcconfigbase.CommonMain(&env, &addr, &token, &envContext, &configRoleSlice[1], &configRoleSlice[0], &tokenName, config)
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
