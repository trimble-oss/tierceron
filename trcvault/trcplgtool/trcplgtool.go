package trcplugtool

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	trcname "tierceron/trcvault/opts/trcname"
	xUtils "tierceron/trcx/xutil"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

func getPluginToolConfig(config *eUtils.DriverConfig, mod *kv.Modifier) map[string]interface{} {
	//templatePaths
	indexFound := false
	templatePaths := []string{}
	for _, startDir := range config.StartDir {
		//get files from directory
		tp := xUtils.GetDirFiles(startDir)
		templatePaths = append(templatePaths, tp...)
	}

	pluginToolConfig, err := mod.ReadData("super-secrets/PluginTool")
	if err != nil {
		eUtils.CheckError(config, err, true)
	}

	for _, templatePath := range templatePaths {
		project, service, _ := eUtils.GetProjectService(templatePath)
		mod.SectionPath = "super-secrets/Index/" + project + "/" + "trcplugin" + "/" + config.SubSectionValue + "/" + service
		ptc1, err := mod.ReadData(mod.SectionPath)
		if err != nil || ptc1 == nil {
			continue
		}
		indexFound = true
		for k, v := range ptc1 {
			pluginToolConfig[k] = v
		}
	}

	if pluginToolConfig == nil || !indexFound {
		eUtils.CheckError(config, errors.New("No plugin configs were found"), true)
	}

	return pluginToolConfig
}

func PluginMain() {
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	envPtr := flag.String("env", "dev", "Environement in vault")
	secretIDPtr := flag.String("secretID", "", "Public app role ID")
	appRoleIDPtr := flag.String("appRoleID", "", "Secret app role ID")
	tokenNamePtr := flag.String("tokenName", "", "Token name used by this tool to access the vault")
	pingPtr := flag.Bool("ping", false, "Ping vault.")
	startDirPtr := flag.String("startDir", trcname.GetFolderPrefix()+"_templates", "Template directory")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+trcname.GetFolderPrefix()+"sub.log", "Output path for log files")
	certifyImagePtr := flag.Bool("certify", false, "Used to certifies vault plugin.")
	pluginNamePtr := flag.String("pluginName", "", "Used to certify vault plugin")
	sha256Ptr := flag.String("sha256", "", "Used to certify vault plugin") //THis has to match the image that is pulled -> then we write the vault.

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		s := args[i]
		if s[0] != '-' {
			fmt.Println("Wrong flag syntax: ", s)
			os.Exit(1)
		}
	}
	flag.Parse()

	// Prints usage if no flags are specified
	if flag.NFlag() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if *certifyImagePtr && (len(*pluginNamePtr) < 0 || len(*sha256Ptr) < 0) {
		fmt.Println("Must use -pluginName && -sha256 flags to use -certify")
		os.Exit(1)
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+trcname.GetFolderPrefix()+"sub.log" {
		*logFilePtr = "./" + trcname.GetFolderPrefix() + "sub.log"
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	logger := log.New(f, "[INIT]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: *insecurePtr, Log: logger, ExitOnFailure: true, StartDir: []string{*startDirPtr}, SubSectionValue: *pluginNamePtr}

	eUtils.CheckError(config, err, true)

	//Grabbing configs
	mod, err := kv.NewModifier(*insecurePtr, *tokenPtr, *addrPtr, *envPtr, nil, logger)
	if err != nil {
		eUtils.CheckError(config, err, true)
	}
	mod.Env = *envPtr

	pluginToolConfig := getPluginToolConfig(config, mod)
	pluginToolConfig["ecrrepository"] = strings.Replace(pluginToolConfig["ecrrepository"].(string), "__imagename__", *pluginNamePtr, -1) //"https://" +
	pluginToolConfig["trcsha256"] = *sha256Ptr
	if *certifyImagePtr {
		svc := ecr.New(session.New(&aws.Config{
			Region:      aws.String("us-west-2"),
			Credentials: credentials.NewStaticCredentials(pluginToolConfig["awspassword"].(string), pluginToolConfig["awsaccesskey"].(string), ""),
		}))
		input := &ecr.BatchGetImageInput{
			ImageIds: []*ecr.ImageIdentifier{
				{
					ImageTag: aws.String("latest"),
				},
			},
			RepositoryName: aws.String(*pluginNamePtr),
			RegistryId:     aws.String(strings.Split(pluginToolConfig["ecrrepository"].(string), ".")[0]),
		}

		result, err := svc.BatchGetImage(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case ecr.ErrCodeServerException:
					fmt.Println(ecr.ErrCodeServerException, aerr.Error())
				case ecr.ErrCodeInvalidParameterException:
					fmt.Println(ecr.ErrCodeInvalidParameterException, aerr.Error())
				case ecr.ErrCodeRepositoryNotFoundException:
					fmt.Println(ecr.ErrCodeRepositoryNotFoundException, aerr.Error())
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
			return
		}
		awsImageSHA := strings.Split(*result.Images[0].ImageId.ImageDigest, ":")[1]
		if pluginToolConfig["trcsha256"].(string) == awsImageSHA {
			//SHA MATCHES
		} else {
			fmt.Println("Invalid or nonexistent image")
			os.Exit(1)
		}
	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)

	autoErr := eUtils.AutoAuth(config, secretIDPtr, appRoleIDPtr, tokenPtr, tokenNamePtr, envPtr, addrPtr, *pingPtr)
	if autoErr != nil {
		fmt.Println("Missing auth components.")
		os.Exit(1)
	}

}
