//go:build azrcr
// +build azrcr

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

func getImageSHA(config *eUtils.DriverConfig, svc *azidentity.ClientSecretCredential, pluginToolConfig map[string]interface{}) error {
	client, err := azcontainerregistry.NewClient(
		pluginToolConfig["acrrepository"].(string),
		svc, nil)
	if err != nil {
		config.Log.Printf("failed to create client: %v", err)
		return err
	}
	ctx := context.Background()
	latestTag := ""
	pager := client.NewListTagsPager(pluginToolConfig["trcplugin"].(string), &azcontainerregistry.ClientListTagsOptions{
		MaxNum:  to.Ptr[int32](1),
		OrderBy: to.Ptr(azcontainerregistry.ArtifactTagOrderByLastUpdatedOnAscending),
	})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			config.Log.Printf("Failed to advance page for tags: %v", err)
			return err
		}
		for _, v := range page.Tags {
			latestTag = *v.Name //Always only returns 1 tag due to MaxNum being set
		}
	}

	// Get manifest
	manifestRes, err := client.GetManifest(ctx, pluginToolConfig["trcplugin"].(string), latestTag, &azcontainerregistry.ClientGetManifestOptions{Accept: to.Ptr(string(azcontainerregistry.ContentTypeApplicationVndDockerDistributionManifestV2JSON))})
	if err != nil {
		config.Log.Printf("failed to get manifest: %v", err)
		return err
	}

	reader, err := azcontainerregistry.NewDigestValidationReader(*manifestRes.DockerContentDigest, manifestRes.ManifestData)
	if err != nil {
		config.Log.Printf("failed to create validation reader: %v", err)
		return err
	}
	manifest, err := io.ReadAll(reader)
	if err != nil {
		config.Log.Printf("failed to read manifest data: %v", err)
		return err
	}

	// Get config
	var manifestJSON map[string]any
	err = json.Unmarshal(manifest, &manifestJSON)
	if err != nil {
		config.Log.Printf("failed to unmarshal manifest: %v", err)
		return err
	}
	// Get layers
	layers := manifestJSON["layers"].([]any)
	blobClient, err := azcontainerregistry.NewBlobClient(pluginToolConfig["acrrepository"].(string), svc, nil)
	if err != nil {
		return errors.New("Failed to create blob client:" + err.Error())
	}

	for i := len(layers) - 1; i > 0; i-- {
		layerD := layers[i].(map[string]any)["digest"].(string)
		sha256, shaErr := GetImageShaFromLayer(blobClient, pluginToolConfig["trcplugin"].(string), layerD, pluginToolConfig)
		if shaErr != nil {
			return errors.New("Failed to load image sha from layer:" + shaErr.Error())
		}
		if pluginToolConfig["trcsha256"].(string) == sha256 {
			break
		}
	}

	return nil
}

func GetImageShaFromLayer(blobClient *azcontainerregistry.BlobClient, name string, digest string, pluginToolConfig map[string]interface{}) (string, error) {

	configRes, err := blobClient.GetBlob(context.Background(), name, digest, nil)
	if err != nil {
		return "", errors.New("Failed to get config:" + err.Error())
	}

	reader, readErr := azcontainerregistry.NewDigestValidationReader(digest, configRes.BlobData)
	if readErr != nil {
		return "", errors.New("Failed to create validation reader" + readErr.Error())
	}

	layerData, configErr := io.ReadAll(reader)
	if configErr != nil {
		return "", errors.New("Failed to read config data:" + configErr.Error())
	}

	pluginTarredData, gUnZipError := gUnZipData(layerData)
	if gUnZipError != nil {
		return "", errors.New("gunzip failed")
	}
	pluginImage, gUnTarError := untarData(pluginTarredData)
	if gUnTarError != nil {
		return "", errors.New("untarring failed")
	}
	pluginSha := sha256.Sum256(pluginImage)
	sha256 := fmt.Sprintf("%x", pluginSha)
	if pluginToolConfig != nil && pluginToolConfig["trcsha256"].(string) == sha256 {
		pluginToolConfig["rawImageFile"] = pluginImage
	}

	return sha256, nil
}

// Return url to the image to be used for download.
func GetImageAndShaFromDownload(config *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	svc, err := azidentity.NewClientSecretCredential(
		pluginToolConfig["azureTenantId"].(string),
		pluginToolConfig["azureClientId"].(string),
		pluginToolConfig["azureClientSecret"].(string),
		nil)
	if err != nil {
		return err
	}

	imageErr := getImageSHA(config, svc, pluginToolConfig)
	if imageErr != nil {
		return imageErr
	}

	return nil
}
