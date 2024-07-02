package trccertmgmtbase

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func CommonMain(flagset *flag.FlagSet, driverConfig *eUtils.DriverConfig, mod *kv.Modifier) error {
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

	openapi, convertErr := openapi2conv.ToV3(&swaggerDoc)
	if convertErr != nil {
		return convertErr
	}

	validateErr := openapi.Validate(context.Background())
	if validateErr != nil {
		return validateErr
	}

	openapiByteArray, err := json.Marshal(openapi)
	openApiString := string(openapiByteArray)
	openApiString = strings.Replace(openApiString, "alpha", "1.0", 1)

	if !strings.Contains(openApiString, `"openapi":"3.0.3","servers": [{"url":"`+apimConfigMap["API_URL"]+`"}]`) {
		openApiString = strings.Replace(openApiString, `"openapi":"3.0.3"`, `"openapi":"3.0.3","servers":[{"url":"`+apimConfigMap["API_URL"]+`"}]`, 1)
		if !strings.Contains(openApiString, apimConfigMap["API_URL"]) {
			return errors.New("Unable to insert server url into apim update.")
		}
	}
	svc, err := azidentity.NewClientSecretCredential(
		apimConfigMap["azureTenantId"],
		apimConfigMap["azureClientId"],
		apimConfigMap["azureClientSecret"],
		nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to obtain a credential: %v", err)
		return err
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	clientFactory, err := armapimanagement.NewClientFactory(apimConfigMap["SUBSCRIPTION_ID"], svc, nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to create client: %v", err)
		return err
	}

	_, eTagErr := clientFactory.NewAPIPolicyClient().GetEntityTag(ctx, apimConfigMap["RESOURCE_GROUP_NAME"], apimConfigMap["SERVICE_NAME"], apimConfigMap["API_NAME"], armapimanagement.PolicyIDNamePolicy, nil)
	if eTagErr != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to finish the request: %v", eTagErr)
		return eTagErr
	}

	t := time.Now().UTC().Format("Monday, 02-Jan-06 15:04:05 MST")

	etag := "*" //Wildcard match on eTag, otherwise it doesn't match from command above.
	poller, err := clientFactory.NewAPIClient().BeginCreateOrUpdate(ctx, apimConfigMap["RESOURCE_GROUP_NAME"], apimConfigMap["SERVICE_NAME"], apimConfigMap["API_NAME"], armapimanagement.APICreateOrUpdateParameter{
		Properties: &armapimanagement.APICreateOrUpdateProperties{
			Path:                   to.Ptr(apimConfigMap["API_PATH"]), //API URL Suffix in portal
			Format:                 to.Ptr(armapimanagement.ContentFormatOpenapiJSON),
			Value:                  to.Ptr(openApiString),
			APIRevisionDescription: to.Ptr(t), //This updates the revision description with current time.
		},
	}, &armapimanagement.APIClientBeginCreateOrUpdateOptions{IfMatch: &etag})
	if err != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to finish the request: %v", err)
		return err
	}

	//Adding a 2 minute timeout on APIM Update.
	go func(ctxC context.CancelFunc) {
		time.Sleep(time.Second * 120)
		ctxC()
	}(ctxCancel)

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to pull the result: %v", err)
		return err
	}

	fmt.Println("Success!")
	_ = resp
	return nil
}
