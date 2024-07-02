//go:build azrcr
// +build azrcr

package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

func getImageSHA(driverConfig *eUtils.DriverConfig, svc *azidentity.ClientSecretCredential, pluginToolConfig map[string]interface{}) error {

	err := ValidateRepository(driverConfig, pluginToolConfig)
	if err != nil {
		return err
	}

	client, err := azcontainerregistry.NewClient(
		pluginToolConfig["acrrepository"].(string),
		svc, nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to create client: %v", err)
		return err
	}
	ctx := context.Background()
	latestTag := ""
	pager := client.NewListTagsPager(pluginToolConfig["trcplugin"].(string), &azcontainerregistry.ClientListTagsOptions{
		MaxNum:  to.Ptr[int32](1),
		OrderBy: to.Ptr(azcontainerregistry.ArtifactTagOrderByLastUpdatedOnDescending),
	})
foundTag:
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			driverConfig.CoreConfig.Log.Printf("Failed to advance page for tags: %v", err)
			return err
		}
		for _, v := range page.Tags {
			latestTag = *v.Name //Always only returns 1 tag due to MaxNum being set
			if latestTag != "" {
				break foundTag
			}
		}
	}

	// Get manifest
	manifestRes, err := client.GetManifest(ctx, pluginToolConfig["trcplugin"].(string), latestTag, &azcontainerregistry.ClientGetManifestOptions{Accept: to.Ptr(string(azcontainerregistry.ContentTypeApplicationVndDockerDistributionManifestV2JSON))})
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to get manifest: %v", err)
		return err
	}

	reader, err := azcontainerregistry.NewDigestValidationReader(*manifestRes.DockerContentDigest, manifestRes.ManifestData)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to create validation reader: %v", err)
		return err
	}
	manifest, err := io.ReadAll(reader)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to read manifest data: %v", err)
		return err
	}

	// Get config
	var manifestJSON map[string]any
	err = json.Unmarshal(manifest, &manifestJSON)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to unmarshal manifest: %v", err)
		return err
	}
	// Get layers
	layers := manifestJSON["layers"].([]any)
	blobClient, err := azcontainerregistry.NewBlobClient(pluginToolConfig["acrrepository"].(string), svc, nil)
	if err != nil {
		return errors.New("Failed to create blob client:" + err.Error())
	}

	for i := len(layers) - 1; i >= 0; i-- {
		if layer, layerOk := layers[i].(map[string]any)["digest"]; layerOk {
			if layerD, ok := layer.(string); ok {
				sha256, shaErr := GetImageShaFromLayer(blobClient, pluginToolConfig["trcplugin"].(string), layerD, pluginToolConfig)
				if shaErr != nil {
					return errors.New("Failed to load image sha from layer:" + shaErr.Error())
				}
				if _, ok := pluginToolConfig["trcsha256"]; !ok {
					// Not looking for anything in particular so just grab the last image.
					break
				} else {
					if pluginToolConfig["trcsha256"].(string) == sha256 {
						pluginToolConfig["imagesha256"] = sha256
						break
					}
				}
			}
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

	pluginTarredData, gUnZipError := gUnZipData(&layerData)
	if gUnZipError != nil {
		return "", errors.New("gunzip failed")
	}
	pluginImage, gUnTarError := untarData(&pluginTarredData)
	if gUnTarError != nil {
		return "", errors.New("untarring failed")
	}
	pluginSha := sha256.Sum256(pluginImage)
	sha256 := fmt.Sprintf("%x", pluginSha)
	if pluginToolConfig != nil {
		if _, ok := pluginToolConfig["trcsha256"]; !ok {
			// Not looking for anything in particular so just grab the last image.
			pluginToolConfig["rawImageFile"] = pluginImage
		} else {
			if pluginToolConfig["trcsha256"].(string) == sha256 {
				pluginToolConfig["rawImageFile"] = pluginImage
			}
		}
	}

	return sha256, nil
}

// Return url to the image to be used for download.
func GetImageAndShaFromDownload(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	svc, err := azidentity.NewClientSecretCredential(
		pluginToolConfig["azureTenantId"].(string),
		pluginToolConfig["azureClientId"].(string),
		pluginToolConfig["azureClientSecret"].(string),
		nil)
	if err != nil {
		return err
	}

	imageErr := getImageSHA(driverConfig, svc, pluginToolConfig)
	if imageErr != nil {
		return imageErr
	}

	return nil
}

// Pushes image to docker registry from: "rawImageFile", and "pluginname" in the map pluginToolConfig.
// https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry#readme-examples
func PushImage(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	err := ValidateRepository(driverConfig, pluginToolConfig)

	if err != nil {
		return err
	}

	svc, err := azidentity.NewClientSecretCredential(
		pluginToolConfig["azureTenantId"].(string),
		pluginToolConfig["azureClientId"].(string),
		pluginToolConfig["azureClientSecret"].(string),
		nil)

	if err != nil {
		return err
	}

	client, err := azcontainerregistry.NewClient(
		pluginToolConfig["acrrepository"].(string),
		svc, nil)

	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to create client: %v", err)
		return err
	}

	blobClient, err := azcontainerregistry.NewBlobClient(
		pluginToolConfig["acrrepository"].(string),
		svc, nil)

	if err != nil {
		driverConfig.CoreConfig.Log.Printf("failed to create blob client: %v", err)
		return err
	}

	ctx := context.Background()
	startRes, err := blobClient.StartUpload(ctx, pluginToolConfig["trcplugin"].(string), nil)

	if err != nil {
		return errors.New("failed to start upload layer: " + err.Error())
	}

	layer := pluginToolConfig["rawImageFile"].([]byte)

	calculator := azcontainerregistry.NewBlobDigestCalculator()
	uploadResp, err := blobClient.UploadChunk(ctx, *startRes.Location, bytes.NewReader(layer), calculator, nil)

	if err != nil {
		return errors.New("failed to upload layer: " + err.Error())
	}

	completeResp, err := blobClient.CompleteUpload(ctx, *uploadResp.Location, calculator, nil)
	if err != nil {
		return errors.New("failed to complete layer upload: " + err.Error())
	}

	layerDigest := *completeResp.DockerContentDigest
	config := []byte(fmt.Sprintf(`{
  architecture: "amd64",
  os: "windows",
  rootfs: {
	type: "layers",
	diff_ids: [%s],
  },
}`, layerDigest))

	startRes, err = blobClient.StartUpload(ctx, pluginToolConfig["trcplugin"].(string), nil)

	if err != nil {
		return errors.New("failed to start upload config: " + err.Error())
	}

	calculator = azcontainerregistry.NewBlobDigestCalculator()
	uploadResp, err = blobClient.UploadChunk(ctx, *startRes.Location, bytes.NewReader(config), calculator, nil)
	if err != nil {
		return errors.New("failed to upload config: " + err.Error())
	}

	completeResp, err = blobClient.CompleteUpload(ctx, *uploadResp.Location, calculator, nil)

	if err != nil {
		return errors.New("failed to complete upload config: " + err.Error())
	}

	manifest := fmt.Sprintf(`
{
	"schemaVersion": 2,
	"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
	"config": {
		"mediaType": "application/vnd.oci.image.config.v1+json",
		"digest": "%s",
		"size": %d
	},
	"layers": [
		{
			"mediaType": "application/vnd.oci.image.layer.v1.tar",
			"digest": "%s",
			"size": %d,
			"annotations": {
				"title": "artifact.txt"
		 	}
		}
	]
}`, layerDigest, len(config), *completeResp.DockerContentDigest, len(layer))

	uploadManifestRes, err := client.UploadManifest(ctx, pluginToolConfig["trcplugin"].(string), "1.0.0",
		azcontainerregistry.ContentTypeApplicationVndDockerDistributionManifestV2JSON, streaming.NopCloser(bytes.NewReader([]byte(manifest))), nil)

	if err != nil {
		return errors.New("failed to upload manifest: " + err.Error())
	}

	driverConfig.CoreConfig.Log.Printf("digest of uploaded manifest: %s", *uploadManifestRes.DockerContentDigest)
	return nil
}

func ValidateRepository(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	if pluginToolConfig["acrrepository"] != nil && len(pluginToolConfig["acrrepository"].(string)) == 0 {
		driverConfig.CoreConfig.Log.Printf("Acr repository undefined.  Refusing to continue.\n")
		return errors.New("undefined acr repository")
	}

	if !strings.HasPrefix(pluginToolConfig["acrrepository"].(string), "https://") {
		driverConfig.CoreConfig.Log.Printf("Malformed Acr repository.  https:// required.  Refusing to continue.\n")
		return errors.New("malformed acr repository - https:// required")
	}

	if pluginToolConfig["trcplugin"] != nil && len(pluginToolConfig["trcplugin"].(string)) == 0 {
		driverConfig.CoreConfig.Log.Printf("Trcplugin undefined.  Refusing to continue.\n")
		return errors.New("undefined trcplugin")
	}

	return nil
}
