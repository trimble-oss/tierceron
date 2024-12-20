package trccertmgmtbase

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/apimanagement/armapimanagement/v2"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func CommonMain(certPathPtr *string, driverConfig *config.DriverConfig, mod *kv.Modifier) error {
	if len(*certPathPtr) == 0 {
		return errors.New("certPath flag is empty, expected path to cert")
	}

	var certBytes []byte
	var err error

	if driverConfig.CoreConfig.IsShell {
		trcshioFile, trcshiFileErr := driverConfig.MemFs.Open(*certPathPtr)
		buffer := bytes.NewBuffer(nil)
		io.Copy(buffer, trcshioFile)
		certBytes = buffer.Bytes()
		err = trcshiFileErr
	} else {
		certBytes, err = os.ReadFile(*certPathPtr)
	}

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
		driverConfig.CoreConfig.Log.Printf("failed to obtain a credential: %v", err)
		return err
	}

	ctx, _ := context.WithCancel(context.Background())
	clientFactory, err := armapimanagement.NewClientFactory(apimConfigMap["SUBSCRIPTION_ID"], svc, nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to create client: %v", err)
		return err
	}

	if resourceGroupName, exists := apimConfigMap["RESOURCE_GROUP_NAME"]; exists {
		if serviceName, exists := apimConfigMap["SERVICE_NAME"]; exists {
			certificateId := time.Now().UTC().Format(strings.ReplaceAll(time.RFC3339, ":", "-"))

			etag := "*"

			_, err = clientFactory.NewCertificateClient().CreateOrUpdate(ctx, resourceGroupName, serviceName, certificateId, armapimanagement.CertificateCreateOrUpdateParameters{
				Properties: &armapimanagement.CertificateCreateOrUpdateProperties{
					Data:     to.Ptr(base64.StdEncoding.EncodeToString(certBytes)),
					Password: to.Ptr(apimConfigMap["CERTIFICATE_PASSWORD"]),
				},
			}, &armapimanagement.CertificateClientCreateOrUpdateOptions{IfMatch: &etag})

			if err != nil {
				driverConfig.CoreConfig.Log.Printf("failed to finish certificate request")
				return err
			}

			fmt.Printf("Certificate %v successfully deployed\n", certificateId)
		} else {
			return errors.New("SERVICE_NAME is not populated in apimConfigMap")
		}
	} else {
		return errors.New("RESOURCE_GROUP_NAME is not populated in apimConfigMap")
	}
	return nil
}
