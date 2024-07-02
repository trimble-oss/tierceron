package trccertmgmtbase

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func CommonMain(flagset *flag.FlagSet, mod *kv.Modifier) error {
	if flagset == nil {
		flagset = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}
	certPathPtr := flagset.String("certPath", "", "Path to certificate to push to Azure")

	if len(*certPathPtr) == 0 {
		return errors.New("certPath flag is empty, expected path to cert")
	}

	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}

	certBytes, err := os.ReadFile(*certPathPtr)
	if err != nil {
		return err
	}

	path, pathErr := os.Getwd()
	if pathErr != nil {
		return pathErr
	}

	swaggerBytes, fileErr := os.ReadFile(path + "/target/swagger.json")
	if fileErr != nil {
		return fileErr
	}

	var swaggerDoc openapi2.T
	swaggerErr := json.Unmarshal(swaggerBytes, &swaggerDoc)
	if swaggerErr != nil {
		return swaggerErr
	}

	apimConfigMap := make(map[string]string)
	tempMap, readErr := mod.ReadData("super-secrets/Restricted/APIMCertConfig/config")
	if readErr != nil {
		return readErr
	} else if len(tempMap) == 0 {
		return errors.New("Couldn't get apim configs for update.")
	}

	for key, value := range tempMap {
		apimConfigMap[fmt.Sprintf("%v", key)] = fmt.Sprintf("%v", value)
	}

}
