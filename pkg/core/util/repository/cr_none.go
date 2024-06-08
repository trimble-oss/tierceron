//go:build !gcr && !azrcr && !awsecr
// +build !gcr,!azrcr,!awsecr

package repository

import (
	"errors"

	// "github.com/docker/docker/client"
	// "github.com/docker/docker/config/auth"
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
	return errors.New("Not defined")

	// TODO: Chewbacca flush out to pull images and download...

	// // Create docker client with options
	// opts := docker.WithHTTPCredentials(auth.NewAuthConfig(auth.AuthConfig{
	// 	Username: pluginToolConfig["dockerUser"].(string),
	// 	Password: pluginToolConfig["dockerPassword"].(string),
	// }))

	// cli, err := client.NewClientWithOpts(client.WithHost(pluginToolConfig["dockerRepository"].(string)), opts)
	// if err != nil {
	// 	panic(err)
	// }

	// images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	// if err != nil {
	// 	return
	// }

	//
	// for _, image := range images {
	// }
}
