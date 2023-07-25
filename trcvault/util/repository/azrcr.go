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
		"https://tierceron.azurecr.io", //pluginToolConfig["ecrrepository"].(string),
		svc, nil)
	if err != nil {
		config.Log.Printf("failed to create client: %v", err)
		return err
	}
	ctx := context.Background()

	// Get manifest
	manifestRes, err := client.GetManifest(ctx, pluginToolConfig["trcplugin"].(string), "latest", &azcontainerregistry.ClientGetManifestOptions{Accept: to.Ptr(string(azcontainerregistry.ContentTypeApplicationVndDockerDistributionManifestV2JSON))})
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
func GetImageDownloadUrl(config *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) (string, error) {
	svc, err := azidentity.NewClientSecretCredential(
		pluginToolConfig["azureSubscriptionId"].(string),
		pluginToolConfig["azureClientId"].(string),
		pluginToolConfig["azureClientSecret"].(string),
		nil)
	// svc, err := azidentity.NewDefaultAzureCredential(&options)
	// if err != nil {
	// 	log.Fatalf("failed to obtain a credential: %v", err)
	// }

	err = getImageSHA(config, svc, pluginToolConfig)
	if err != nil {
		return "", err
	}

	return "", nil
	// blobClient, err := azcontainerregistry.NewBlobClient("<your Container Registry's endpoint URL>", svc, nil)
	// if err != nil {
	// 	log.Fatalf("failed to create blob client: %v", err)
	// }

	// configRes, err := blobClient.GetBlob(ctx, pluginToolConfig["trcplugin"].(string), configDigest, nil)
	// if err != nil {
	// 	log.Fatalf("failed to get config: %v", err)
	// }
	// reader, err = azcontainerregistry.NewDigestValidationReader(configDigest, configRes.BlobData)
	// if err != nil {
	// 	log.Fatalf("failed to create validation reader: %v", err)
	// }
	// config, err := io.ReadAll(reader)
	// if err != nil {
	// 	log.Fatalf("failed to read config data: %v", err)
	// }
	// fmt.Printf("config: %s\n", config)

	// // Get layers
	// layers := manifestJSON["layers"].([]any)
	// for _, layer := range layers {
	// 	layerDigest := layer.(map[string]any)["digest"].(string)
	// 	layerRes, err := blobClient.GetBlob(ctx, pluginToolConfig["trcplugin"].(string), layerDigest, nil)
	// 	if err != nil {
	// 		log.Fatalf("failed to get layer: %v", err)
	// 	}
	// 	reader, err = azcontainerregistry.NewDigestValidationReader(layerDigest, layerRes.BlobData)
	// 	if err != nil {
	// 		log.Fatalf("failed to create validation reader: %v", err)
	// 	}
	// 	image, err := io.ReadAll(reader)
	// 	if err != nil {
	// 		log.Fatalf("Failed to read layer: %v", err)
	// 	}
	// }

	// // Get the registry download URL for the layer.
	// downloadURL, err := image.GetLayerDownloadURL(context.Background(), pluginToolConfig["layerDigest"].(string))
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// return downloadUrl, nil
}

func GetImageAndShaFromDownload(config *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	downloadUrl, downloadURlError := GetImageDownloadUrl(config, pluginToolConfig)
	if downloadURlError != nil {
		return errors.New("Failed to get download url.")
	}
	pluginImageDataCompressed, downloadError := getImage(downloadUrl)
	if downloadError != nil {
		return errors.New("Failed to get download from url.")
	}
	pluginTarredData, gUnZipError := gUnZipData(pluginImageDataCompressed)
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
