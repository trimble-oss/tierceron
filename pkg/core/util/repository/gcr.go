//go:build gcr
// +build gcr

package repository

import (
	"errors"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

func getImageSHA(pluginToolConfig map[string]any) error {
	// TODO: implement
	return nil
}

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]any) (string, error) {
	// TODO: implement
	return "", nil
}

func GetImageAndShaFromDownload(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]any) error {
	return errors.New("Not defined")
}

// Pushes image to docker registry from: "rawImageFile", and "pluginname" in the map pluginToolConfig.
func PushImage(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]any) error {
	return errors.New("Not defined")
}
