package trcplugtool

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	trcname "tierceron/trcvault/opts/trcname"
	"tierceron/trcvault/util/repository"
	xUtils "tierceron/trcx/xutil"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"
)

func getPluginToolConfig(config *eUtils.DriverConfig, mod *kv.Modifier, pluginName string, sha string) map[string]interface{} {
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
		break
	}

	if pluginToolConfig == nil || !indexFound {
		eUtils.CheckError(config, errors.New("No plugin configs were found"), true)
	}

	pluginToolConfig["ecrrepository"] = strings.Replace(pluginToolConfig["ecrrepository"].(string), "__imagename__", pluginName, -1) //"https://" +
	pluginToolConfig["trcsha256"] = sha
	pluginToolConfig["pluginNamePtr"] = pluginName
	return pluginToolConfig
}

func gUnZipData(data []byte) ([]byte, error) {
	var unCompressedBytes []byte
	newB := bytes.NewBuffer(unCompressedBytes)
	b := bytes.NewBuffer(data)
	zr, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(newB, zr); err != nil {
		return nil, err
	}

	return newB.Bytes(), nil
}

func untarData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	writer := io.Writer(&b)
	tarReader := tar.NewReader(bytes.NewReader(data))
	for {
		_, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(writer, tarReader)
		if err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

func getImage(downloadUrl string) ([]byte, error) {
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func PluginMain() {
	addrPtr := flag.String("addr", "", "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	envPtr := flag.String("env", "dev", "Environement in vault")
	startDirPtr := flag.String("startDir", trcname.GetFolderPrefix()+"_templates", "Template directory")
	insecurePtr := flag.Bool("insecure", false, "By default, every ssl connection is secure.  Allows to continue with server connections considered insecure.")
	logFilePtr := flag.String("log", "./"+trcname.GetFolderPrefix()+"plgtool.log", "Output path for log files")
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

	if *certifyImagePtr && (len(*pluginNamePtr) == 0 || len(*sha256Ptr) == 0) {
		fmt.Println("Must use -pluginName && -sha256 flags to use -certify")
		os.Exit(1)
	}

	//Ensure that ptr has required suffix
	if *sha256Ptr != "" {
		if !strings.HasPrefix(*sha256Ptr, "sha256:") {
			*sha256Ptr = "sha256:" + *sha256Ptr
		}
	}

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+trcname.GetFolderPrefix()+"plgtool.log" {
		*logFilePtr = "./" + trcname.GetFolderPrefix() + "plgtool.log"
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
	pluginToolConfig := getPluginToolConfig(config, mod, *pluginNamePtr, *sha256Ptr)

	//Certify Image
	if *certifyImagePtr {
		downloadUrl, downloadURlError := repository.GetImageDownloadUrl(pluginToolConfig)
		if downloadURlError != nil {
			fmt.Println("Failed to get download url.")
			os.Exit(1)
		}
		pluginImageDataCompressed, downloadError := getImage(downloadUrl)
		if downloadError != nil {
			fmt.Println("Failed to get download from url.")
			os.Exit(1)
		}
		pluginTarredData, gUnZipError := gUnZipData(pluginImageDataCompressed)
		if gUnZipError != nil {
			fmt.Println("gUnZip failed.")
			os.Exit(1)
		}
		pluginImage, gUnTarError := untarData(pluginTarredData)
		if gUnTarError != nil {
			fmt.Println("Untarring failed.")
			os.Exit(1)
		}
		pluginSha := sha256.Sum256(pluginImage)
		pluginToolConfig["imagesha256"] = fmt.Sprintf("sha256:%x", pluginSha)
		if pluginToolConfig["trcsha256"].(string) == pluginToolConfig["imagesha256"].(string) { //Comparing generated sha from image to sha from flag
			fmt.Println("Valid image found.")
			//SHA MATCHES
		} else {
			fmt.Println("Invalid or nonexistent image.")
			os.Exit(1)
		}

	}

	fmt.Printf("Connecting to vault @ %s\n", *addrPtr)
	writeMap := make(map[string]interface{})
	writeMap["trcplugin"] = pluginToolConfig["trcplugin"].(string)
	writeMap["trcsha256"] = strings.TrimPrefix(pluginToolConfig["trcsha256"].(string), "sha256:") //Trimming so it matches original format
	_, err = mod.Write(mod.SectionPath, writeMap)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Image certified in vault and is ready for release.")
}
