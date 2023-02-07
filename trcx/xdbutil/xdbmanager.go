package xdbutil

import (
	"os"

	trcdb "github.com/trimble-oss/tierceron/trcx/db"
	"github.com/trimble-oss/tierceron/trcx/xutil"
	eUtils "github.com/trimble-oss/tierceron/utils"
)

// GenerateSeedsFromVaultToDb pulls all data from vault for each template into a database
func GenerateSeedsFromVaultToDb(config *eUtils.DriverConfig) (interface{}, error) {
	if config.Diff { //Clean flag in trcx
		_, err1 := os.Stat(config.EndDir + config.Env)
		err := os.RemoveAll(config.EndDir + config.Env)

		if err != nil {
			eUtils.LogErrorObject(config, err, false)
			return nil, err
		}

		if err1 == nil {
			eUtils.LogInfo(config, "Seed removed from"+config.EndDir+config.Env)
		}
		return nil, nil
	}

	// Get files from directory
	tempTemplatePaths := []string{}
	for _, startDir := range config.StartDir {
		//get files from directory
		tp := xutil.GetDirFiles(startDir)
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

	tierceronEngine, err := trcdb.CreateEngine(config,
		templatePaths, config.Env, config.VersionFilter[0])
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}

	return tierceronEngine, nil
}
