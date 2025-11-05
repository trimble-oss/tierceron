package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	hccore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trchealthcheck/hcore"

	// Update package path as needed
	"gopkg.in/yaml.v2"
)

func GetConfigPaths(pluginName string) []string {
	return hccore.GetConfigPaths(pluginName)
}

func Init(pluginName string, properties *map[string]interface{}) {
	hccore.Init(pluginName, properties)
}

func main() {
	logFilePtr := flag.String("log", "./trchealthcheck.log", "Output path for log file")
	flag.Parse()
	config := make(map[string]interface{})

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trchealthcheck]", log.LstdFlags)
	config["log"] = logger

	data, err := os.ReadFile("config.yml")
	if err != nil {
		logger.Println("Error reading YAML file:", err)
		os.Exit(-1)
	}

	// Create an empty map for the YAML data
	var configCommon map[string]interface{}

	// Unmarshal the YAML data into the map
	err = yaml.Unmarshal(data, &configCommon)
	if err != nil {
		logger.Println("Error unmarshaling YAML:", err)
		os.Exit(-1)
	}
	config[hccore.COMMON_PATH] = &configCommon

	serviceCertBytes, err := os.ReadFile(fmt.Sprintf("./local_config/%s", tccore.TRCSHHIVEK_CERT))
	if err != nil {
		logger.Printf("Couldn't load cert: %v", err)
	}

	serviceKeyBytes, err := os.ReadFile(fmt.Sprintf("./local_config/%s", tccore.TRCSHHIVEK_KEY))
	if err != nil {
		logger.Printf("Couldn't load key: %v", err)
	}
	config[tccore.TRCSHHIVEK_CERT] = serviceCertBytes
	config[tccore.TRCSHHIVEK_KEY] = serviceKeyBytes

	Init("healthcheck", &config)

	hccore.GetConfigContext("healthcheck").Start("healthcheck")
	time.Sleep(5 * time.Second)
	msg := hccore.HelloWorldDiagnostic()
	fmt.Fprintln(os.Stderr, msg)
	wait := make(chan bool)
	wait <- true
}
