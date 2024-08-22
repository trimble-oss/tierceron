//go:build azrcr
// +build azrcr

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
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
	for _, key := range []string{"azureTenantId", "azureClientId", "azureClientSecret"} {
		if val, ok := pluginToolConfig[key]; !ok || len(val.(string)) == 0 {
			driverConfig.CoreConfig.Log.Printf("ACR credential %v undefined. Refusing to continue.\n", key)
			return fmt.Errorf("undefined ACR credential: %v", key)
		}
	}

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return errors.New("failed to create Docker client: " + err.Error())
	}

	imageNameTag := pluginToolConfig["pluginNamePtr"].(string)
	repo := strings.TrimPrefix(pluginToolConfig["acrrepository"].(string), "https://")
	qualifiedName := fmt.Sprintf("%s/%s", repo, imageNameTag)

	err = dockerCli.ImageTag(context.Background(), imageNameTag, qualifiedName)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Failed to tag image: %v\n", err)
		return err
	}

	authConfig := registry.AuthConfig{
		Username: pluginToolConfig["azureClientId"].(string),
		Password: pluginToolConfig["azureClientSecret"].(string),
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Failed to encode Docker auth configuration: %v\n", err)
		return err
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	pushResponse, err := dockerCli.ImagePush(context.Background(), qualifiedName, image.PushOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Failed to push image to ACR: %v\n", err)
		return err
	}
	defer pushResponse.Close()

	_, err = io.Copy(os.Stdout, pushResponse)
	if err != nil {
		driverConfig.CoreConfig.Log.Printf("Failed to read push response: %v\n", err)
		return err
	}

	driverConfig.CoreConfig.Log.Printf("Image %s pushed to ACR successfully.\n", imageNameTag)

	deleteDockerImage(imageNameTag)
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
