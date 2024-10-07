package xdbutil

import (
	"os"

	"github.com/trimble-oss/tierceron/pkg/utils/config"

	trcdb "github.com/trimble-oss/tierceron/atrium/trcdb"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// GenerateSeedsFromVaultToDb pulls all data from vault for each template into a database
func GenerateSeedsFromVaultToDb(driverConfig *config.DriverConfig) (interface{}, error) {
	if driverConfig.Diff { //Clean flag in trcx
		_, err1 := os.Stat(driverConfig.EndDir + driverConfig.CoreConfig.Env)
		err := os.RemoveAll(driverConfig.EndDir + driverConfig.CoreConfig.Env)

		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			return nil, err
		}

		if err1 == nil {
			eUtils.LogInfo(driverConfig.CoreConfig, "Seed removed from"+driverConfig.EndDir+driverConfig.CoreConfig.Env)
		}
		return nil, nil
	}

	// Get files from directory
	tempTemplatePaths := []string{}
	for _, startDir := range driverConfig.StartDir {
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

	tierceronEngine, err := trcdb.CreateEngine(driverConfig,
		templatePaths, driverConfig.CoreConfig.Env, driverConfig.VersionFilter[0])
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}

	return tierceronEngine, nil
}
