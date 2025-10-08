package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	core "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trctrcsh/core"
)

func GetConfigPaths(pluginName string) []string {
	return core.GetConfigPaths(pluginName)
}

func Init(pluginName string, properties *map[string]any) {
	core.Init(pluginName, properties)
}

func main() {
	logFilePtr := flag.String("log", "./trchelloworld.log", "Output path for log file")
	flag.Parse()
	config := make(map[string]any)

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trchelloworld]", log.LstdFlags)
	config["log"] = logger

	data, err := os.ReadFile("config.yml")
	if err != nil {
		logger.Println("Error reading YAML file:", err)
		os.Exit(-1)
	}

	// Create an empty map for the YAML data
	var configCommon map[string]any

	// Unmarshal the YAML data into the map
	err = yaml.Unmarshal(data, &configCommon)
	if err != nil {
		logger.Println("Error unmarshaling YAML:", err)
		os.Exit(-1)
	}

	Init("trcsh", &config)
	core.GetConfigContext("trcsh").Start("trcsh")
}
