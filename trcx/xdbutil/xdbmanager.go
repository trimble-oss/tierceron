package xdbutil

import (
	"fmt"
	"os"
	xdb "tierceron/trcx/db"
	xUtils "tierceron/trcx/xutil"
	eUtils "tierceron/utils"
)

// GenerateSeedsFromVaultToDb pulls all data from vault for each template into a database
func GenerateSeedsFromVaultToDb(config eUtils.DriverConfig) interface{} {
	if config.Diff { //Clean flag in trcx
		_, err1 := os.Stat(config.EndDir + config.Env)
		err := os.RemoveAll(config.EndDir + config.Env)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err1 == nil {
			fmt.Println("Seed removed from", config.EndDir+config.Env)
		}
		return nil
	}

	// Get files from directory
	tempTemplatePaths := []string{}
	for _, startDir := range config.StartDir {
		//get files from directory
		tp := xUtils.GetDirFiles(startDir)
		tempTemplatePaths = append(tempTemplatePaths, tp...)
	}

	//Duplicate path remover
	keys := make(map[string]bool)
	templatePaths := []string{}
	for _, path := range tempTemplatePaths {
		if _, value := keys[path]; !value {
			keys[path] = true
			templatePaths = append(templatePaths, path)
		}
	}

	tierceronEngine, err := xdb.CreateEngine(&config,
		templatePaths, config.Env, config.VersionFilter[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return tierceronEngine
}
