//go:build dockercr

package repository

import (
	"context"
	"errors"

	"github.com/moby/moby/api/pkg/authconfig"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]any) (string, error) {
	return "", nil
}

// Defines the keys: "rawImageFile", and "imagesha256" in the map pluginToolConfig.
// TODO: make this scale by streaming image to disk
// (maybe parameterizable so only activated for known larger deployment images)
func GetImageAndShaFromDownload(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]any) error {
	// TODO: Chewbacca flush out to pull images and download...

	dockerAuth := registry.AuthConfig{
		Username: pluginToolConfig["dockerUser"].(string),
		Password: pluginToolConfig["dockerPassword"].(string),
	}

	cli, err := client.New(client.WithHost(pluginToolConfig["dockerRepository"].(string)))
	if err != nil {
		panic(err)
	}
	//ctx := context.Background()
	// token, err := cli.RegistryLogin(ctx, dockerAuth)
	// if err != nil {
	// 	return err
	// }
	// dockerAuth.IdentityToken = token.IdentityToken
	auth, err := authconfig.Encode(dockerAuth)
	if err != nil {
		return err
	}

	opts := client.ImagePullOptions{
		RegistryAuth: auth,
	}

	images, err := cli.ImageList(context.Background(), client.ImageListOptions{})
	if err != nil {
		return err
	}

	for _, image := range images.Items {
		pullResponse, err := cli.ImagePull(context.Background(), image.ID, opts)
		if err != nil {
			return err
		}
		_ = pullResponse.Close()
	}
	return nil
}

// Pushes image to docker registry from: "rawImageFile", and "pluginname" in the map pluginToolConfig.
func PushImage(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]any) error {
	return errors.New("Not defined")
}
