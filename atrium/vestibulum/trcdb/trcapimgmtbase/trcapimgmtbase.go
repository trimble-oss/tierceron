package trcapimgmtbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func CommonMain(envPtr *string,
	envCtxPtr *string,
	tokenNamePtr *string,
	regionPtr *string,
	startDirPtr *string,
	driverConfig *config.DriverConfig,
	mod *kv.Modifier) error {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
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
	tempMap, readErr := mod.ReadData("super-secrets/Restricted/APIMConfig/config")
	if readErr != nil {
		return readErr
	} else if len(tempMap) == 0 {
		return errors.New("Couldn't get apim configs for update.")
	}

	for key, value := range tempMap {
		apimConfigMap[fmt.Sprintf("%v", key)] = fmt.Sprintf("%v", value)
	}

	if title, titleOK := apimConfigMap["API_TITLE"]; titleOK && title != "" {
		swaggerDoc.Info.Title = apimConfigMap["API_TITLE"]
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

	//ApiManagementCreateApiUsingSwaggerImport
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
