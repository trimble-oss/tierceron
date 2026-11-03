package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	hcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchiveye/hcore"
	"gopkg.in/yaml.v2"
)

func GetConfigPaths(pluginName string) []string {
	return hcore.GetConfigPaths(pluginName)
}

func Init(pluginName string, properties *map[string]any) {
	hcore.Init(pluginName, properties)
}

func main() {
	logFilePtr := flag.String("log", "./trchiveye.log", "Output path for log file")
	flag.Parse()
	config := make(map[string]any)

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trchiveye]", log.LstdFlags)
	config["log"] = logger

	data, err := os.ReadFile("config.yml")
	if err != nil {
		logger.Println("Error reading YAML file:", err)
		os.Exit(-1)
	}

	var configCommon map[string]any
	err = yaml.Unmarshal(data, &configCommon)
	if err != nil {
		logger.Println("Error unmarshaling YAML:", err)
		os.Exit(-1)
	}
	config[hcore.COMMON_PATH] = &configCommon

	helloCertBytes, err := os.ReadFile("./hello.crt")
	if err != nil {
		log.Printf("Couldn't load cert: %v", err)
	}

	helloKeyBytes, err := os.ReadFile("./hellokey.key")
	if err != nil {
		log.Printf("Couldn't load key: %v", err)
	}
	config[tccore.TRCSHHIVEK_CERT] = helloCertBytes
	config[tccore.TRCSHHIVEK_KEY] = helloKeyBytes

	Init("hiveye", &config)
	hcore.GetConfigContext("hiveye").Start("hiveye")
}
