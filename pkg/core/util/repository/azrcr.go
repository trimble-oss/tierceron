//go:build azrcr
// +build azrcr

package repository

import (
	"archive/tar"
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
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

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

	// Check for required ACR credentials
	if pluginToolConfig["azureTenantId"] == nil || pluginToolConfig["azureClientId"] == nil || pluginToolConfig["azureClientSecret"] == nil {
		driverConfig.CoreConfig.Log.Printf("ACR credentials undefined. Refusing to continue.\n")
		return errors.New("undefined ACR credentials")
	}
	if len(pluginToolConfig["azureTenantId"].(string)) == 0 || len(pluginToolConfig["azureClientId"].(string)) == 0 || len(pluginToolConfig["azureClientSecret"].(string)) == 0 {
		driverConfig.CoreConfig.Log.Printf("ACR credentials undefined. Refusing to continue.\n")
		return errors.New("undefined ACR credentials")
	}

	cred, err := azidentity.NewClientSecretCredential(
		pluginToolConfig["azureTenantId"].(string),
		pluginToolConfig["azureClientId"].(string),
		pluginToolConfig["azureClientSecret"].(string),
		nil,
	)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Failed to authenticate with Azure: %v\n", err)
		return err
	}

	azrClient, err := azcontainerregistry.NewClient(pluginToolConfig["acrrepository"].(string), cred, nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Failed to create ACR client: %v\n", err)
		return err
	}

	blobClient, err := azcontainerregistry.NewBlobClient(pluginToolConfig["acrrepository"].(string), cred, nil)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Failed to create ACR Blob client: %v\n", err)
		return err
	}

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return errors.New("failed to create Docker client: " + err.Error())
	}

	imageNameTag := pluginToolConfig["pluginNamePtr"].(string)
	imageName := strings.Split(imageNameTag, ":")[0]

	imageReader, err := dockerCli.ImageSave(context.Background(), []string{imageNameTag})
	if err != nil {
		return errors.New("failed to save Docker image: " + err.Error())
	}
	defer imageReader.Close()

	// Read the tarball and process layers
	tarReader := tar.NewReader(imageReader)

	layerDigests := []string{}
	layerSizes := []int64{}
	var configData []byte

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("failed to read Docker image tarball: " + err.Error())
		}

		driverConfig.CoreConfig.Log.Printf("Processing header %v", header.Name)
		// Process layers
		if strings.HasSuffix(header.Name, "/layer.tar") {
			layerData, err := io.ReadAll(tarReader)
			if err != nil {
				return errors.New("failed to read layer data: " + err.Error())
			}

			ctx := context.Background()
			startRes, err := blobClient.StartUpload(ctx, imageName, nil)
			if err != nil {
				return errors.New("failed to start layer upload: " + err.Error())
			}

			calculator := azcontainerregistry.NewBlobDigestCalculator()
			uploadResp, err := blobClient.UploadChunk(ctx, *startRes.Location, bytes.NewReader(layerData), calculator, nil)
			if err != nil {
				return errors.New("failed to upload layer: " + err.Error())
			}

			completeResp, err := blobClient.CompleteUpload(ctx, *uploadResp.Location, calculator, nil)
			if err != nil {
				return errors.New("failed to complete layer upload: " + err.Error())
			}

			layerDigests = append(layerDigests, *completeResp.DockerContentDigest)
			layerSizes = append(layerSizes, header.Size)
		} else {
			driverConfig.CoreConfig.Log.Printf("Skipping non-tar header %v", header.Name)
		}

		// Process config
		if strings.HasSuffix(header.Name, ".json") && strings.Contains(header.Name, "config") {
			configData, err = io.ReadAll(tarReader)
			if err != nil {
				return errors.New("failed to read config data: " + err.Error())
			}
		}
	}

	if len(layerSizes) == 0 || len(layerSizes) != len(layerDigests) {
		return fmt.Errorf("unsupported # of layer sizes / digest array, %d %d", len(layerSizes), len(layerDigests))
	}

	// Upload config
	startRes, err := blobClient.StartUpload(context.Background(), imageName, nil)
	if err != nil {
		return errors.New("failed to start config upload: " + err.Error())
	}

	calculator := azcontainerregistry.NewBlobDigestCalculator()
	uploadResp, err := blobClient.UploadChunk(context.Background(), *startRes.Location, bytes.NewReader(configData), calculator, nil)
	if err != nil {
		return errors.New("failed to upload config: " + err.Error())
	}

	completeResp, err := blobClient.CompleteUpload(context.Background(), *uploadResp.Location, calculator, nil)
	if err != nil {
		return errors.New("failed to complete config upload: " + err.Error())
	}

	configDigest := *completeResp.DockerContentDigest

	// Generate the manifest
	layers := []map[string]interface{}{}
	for i, digest := range layerDigests {
		layers = append(layers, map[string]interface{}{
			"mediaType": "application/vnd.oci.image.layer.v1.tar",
			"digest":    digest,
			"size":      layerSizes[i],
		})
	}

	manifest := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.docker.distribution.manifest.v2+json",
		"config": map[string]interface{}{
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest":    configDigest,
			"size":      len(configData),
		},
		"layers": layers,
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return errors.New("failed to marshal manifest: " + err.Error())
	}

	tag := "latest"
	nameParts := strings.Split(pluginToolConfig["pluginNamePtr"].(string), ":")
	if len(nameParts) == 2 {
		tag = nameParts[1]
	}

	uploadManifestRes, err := azrClient.UploadManifest(context.Background(), imageName, tag,
		azcontainerregistry.ContentTypeApplicationVndDockerDistributionManifestV2JSON, streaming.NopCloser(bytes.NewReader(manifestData)), nil)
	if err != nil {
		return errors.New("failed to upload manifest: " + err.Error())
	}

	driverConfig.CoreConfig.Log.Printf("Digest of uploaded manifest: %s", *uploadManifestRes.DockerContentDigest)

	// Clean up local Docker image
	err = deleteDockerImage(imageNameTag)
	if err != nil {
		return errors.New("failed to delete local Docker image: " + err.Error())
	}

	return nil
}

func deleteDockerImage(imageName string) error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	_, err = cli.ImageRemove(ctx, imageName, image.RemoveOptions{
		Force:         false,
		PruneChildren: true,
	})
	return err
}

func ValidateRepository(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	if val, ok := pluginToolConfig["acrrepository"]; !ok || len(val.(string)) == 0 {
		driverConfig.CoreConfig.Log.Printf("Acr repository undefined.  Refusing to continue.\n")
		return errors.New("undefined acr repository")
	}

	if !strings.HasPrefix(pluginToolConfig["acrrepository"].(string), "https://") {
		driverConfig.CoreConfig.Log.Printf("Malformed Acr repository.  https:// required.  Refusing to continue.\n")
		return errors.New("malformed acr repository - https:// required")
	}

	if val, ok := pluginToolConfig["pluginNamePtr"]; !ok || len(val.(string)) == 0 {
		driverConfig.CoreConfig.Log.Printf("pluginNamePtr undefined.  Refusing to continue.\n")
		return errors.New("undefined pluginNamePtr")
	}
	return nil
}
