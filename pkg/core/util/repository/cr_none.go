//go:build !gcr && !azrcr && !awsecr && !dockercr
// +build !gcr,!azrcr,!awsecr,!dockercr

package repository

import (
	"errors"

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
}
