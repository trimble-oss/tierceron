package trccertmgmtbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

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

func CommonMain(certPathPtr *string, driverConfig *eUtils.DriverConfig, mod *kv.Modifier) error {
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

	ctx, _ := context.WithCancel(context.Background())
	clientFactory, err := armapimanagement.NewClientFactory(apimConfigMap["SUBSCRIPTION_ID"], svc, nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to create client: %v", err)
		return err
	}

	resourceGroupName := apimConfigMap["RESOURCE_GROUP_NAME"]
	serviceName := apimConfigMap["SERVICE_NAME"]
	certificateId := apimConfigMap["CERTIFICATE_ID"]

	_, eTagErr := clientFactory.NewCertificateClient().GetEntityTag(ctx, resourceGroupName, serviceName, certificateId, nil)
	if eTagErr != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to finish the request: %v", eTagErr)
		return eTagErr
	}

	etag := "*" //Wildcard match on eTag, otherwise it doesn't match from command above.

	keyVault := &armapimanagement.KeyVaultContractCreateProperties{
		IdentityClientID: nil,
		SecretIdentifier: nil,
	}

	_, err = clientFactory.NewCertificateClient().CreateOrUpdate(ctx, resourceGroupName, serviceName, certificateId, armapimanagement.CertificateCreateOrUpdateParameters{
		Properties: &armapimanagement.CertificateCreateOrUpdateProperties{
			Data:     to.Ptr(string(certBytes)),
			KeyVault: keyVault,
			Password: to.Ptr(apimConfigMap["CERTIFICATE_PASSWORD"]),
		},
	}, &armapimanagement.CertificateClientCreateOrUpdateOptions{IfMatch: &etag})

	if err != nil {
		driverConfig.CoreConfig.Log.Fatalf("failed to finish the request: %v", err)
		return err
	}

	fmt.Println("Success!")
	return nil
}
