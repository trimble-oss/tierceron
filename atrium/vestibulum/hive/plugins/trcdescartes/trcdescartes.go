package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	hcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcdescartes/hcore"
	"gopkg.in/yaml.v2"
	// Update package path as needed
)

func GetConfigPaths(pluginName string) []string {
	return hcore.GetConfigPaths(pluginName)
}

func Init(pluginName string, properties *map[string]any) {
	hcore.Init(pluginName, properties)
}

func main() {
	logFilePtr := flag.String("log", "./trcdescartes.log", "Output path for log file")
	flag.Parse()
	config := make(map[string]any)

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trcdescartes]", log.LstdFlags)
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
	config[hcore.COMMON_PATH] = &configCommon

	Init("descartes", &config)
	hcore.GetConfigContext("descartes").Start("descartes")
}
