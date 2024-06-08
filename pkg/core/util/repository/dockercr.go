//go:build dockercr
// +build dockercr

package repository

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	return "", nil
}

// Defines the keys: "rawImageFile", and "imagesha256" in the map pluginToolConfig.
// TODO: make this scale by streaming image to disk
// (maybe parameterizable so only activated for known larger deployment images)
func GetImageAndShaFromDownload(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	// TODO: Chewbacca flush out to pull images and download...

	opts := docker.WithHTTPCredentials(auth.NewAuthConfig(auth.AuthConfig{
		Username: pluginToolConfig["dockerUser"].(string),
		Password: pluginToolConfig["dockerPassword"].(string),
	}))

	cli, err := client.NewClientWithOpts(client.WithHost(pluginToolConfig["dockerRepository"].(string)), opts)
	if err != nil {
		panic(err)
	}

	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		return
	}

	for _, image := range images {
	}
}
