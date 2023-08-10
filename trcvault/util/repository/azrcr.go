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
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"

	eUtils "github.com/trimble-oss/tierceron/utils"
)

func getImageSHA(config *eUtils.DriverConfig, svc *azidentity.ClientSecretCredential, pluginToolConfig map[string]interface{}) error {
	client, err := azcontainerregistry.NewClient(
		pluginToolConfig["acrrepository"].(string),
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
			log.Fatalf("failed to advance page for tags: %v", err)
		}
		for _, v := range page.Tags {
			latestTag = *v.Name //Always only returns 1 tag due to MaxNum being set
		}
	}

	// Get manifest
	manifestRes, err := client.GetManifest(ctx, pluginToolConfig["trcplugin"].(string), latestTag, &azcontainerregistry.ClientGetManifestOptions{Accept: to.Ptr(string(azcontainerregistry.ContentTypeApplicationVndDockerDistributionManifestV2JSON))})
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
	var layerDigest string
	layers := manifestJSON["layers"].([]any)
	for _, layer := range layers {
		layerDigest = layer.(map[string]any)["digest"].(string)
	}

	pluginToolConfig["layerDigest"] = layerDigest
	return nil
}

// Return url to the image to be used for download.
func GetImageAndShaFromDownload(config *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
func GetImageAndShaFromDownload(config *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	svc, err := azidentity.NewClientSecretCredential(
		pluginToolConfig["azureTenantId"].(string),
		pluginToolConfig["azureTenantId"].(string),
		pluginToolConfig["azureClientId"].(string),
		pluginToolConfig["azureClientSecret"].(string),
		nil)

	imageErr := getImageSHA(config, svc, pluginToolConfig)
	if imageErr != nil {
		return imageErr
		return imageErr
	}
	blobClient, err := azcontainerregistry.NewBlobClient(pluginToolConfig["acrrepository"].(string), svc, nil)
	if err != nil {
		log.Fatalf("failed to create blob client: %v", err)
	}

	configRes, err := blobClient.GetBlob(context.Background(), pluginToolConfig["trcplugin"].(string), pluginToolConfig["layerDigest"].(string), nil)
	if err != nil {
		log.Fatalf("failed to get config: %v", err)
	}

	reader, readErr := azcontainerregistry.NewDigestValidationReader(pluginToolConfig["layerDigest"].(string), configRes.BlobData)
	if readErr != nil {
		log.Fatalf("failed to create validation reader: %v", readErr)
	}


	layerData, configErr := io.ReadAll(reader)
	if configErr != nil {
		log.Fatalf("failed to read config data: %v", configErr)
	}

	pluginTarredData, gUnZipError := gUnZipData(layerData)
	pluginTarredData, gUnZipError := gUnZipData(layerData)
	if gUnZipError != nil {
		return errors.New("Gunzip failed.")
	}
	pluginImage, gUnTarError := untarData(pluginTarredData)
	if gUnTarError != nil {
		return errors.New("Untarring failed.")
	}
	pluginSha := sha256.Sum256(pluginImage)
	pluginToolConfig["rawImageFile"] = pluginImage
	pluginToolConfig["imagesha256"] = fmt.Sprintf("%x", pluginSha)
	return nil
}
