//go:build !gcr && !azrcr && !awsecr && !dockercr
// +build !gcr,!azrcr,!awsecr,!dockercr

package repository

import (
	"errors"

	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	return "", nil
}

// Defines the keys: "rawImageFile", and "imagesha256" in the map pluginToolConfig.
// TODO: make this scale by streaming image to disk
// (maybe parameterizable so only activated for known larger deployment images)
func GetImageAndShaFromDownload(driverConfig *config.DriverConfig, pluginToolConfig map[string]interface{}) error {
	return errors.New("Not defined")
}

// Pushes image to docker registry from: "rawImageFile", and "pluginname" in the map pluginToolConfig.
func PushImage(driverConfig *config.DriverConfig, pluginToolConfig map[string]interface{}) error {
	// TODO: implement push image to registry.
	return errors.New("Not defined")
}
