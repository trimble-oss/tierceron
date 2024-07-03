package trccertmgmtbase

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
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
	certificateId := time.Now().UTC().Format(strings.ReplaceAll(time.RFC3339, ":", "-"))

	etag := "*" //Wildcard match on eTag, otherwise it doesn't match from command above.

	_, err = clientFactory.NewCertificateClient().CreateOrUpdate(ctx, resourceGroupName, serviceName, certificateId, armapimanagement.CertificateCreateOrUpdateParameters{
		Properties: &armapimanagement.CertificateCreateOrUpdateProperties{
			Data:     to.Ptr(base64.StdEncoding.EncodeToString(certBytes)),
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
