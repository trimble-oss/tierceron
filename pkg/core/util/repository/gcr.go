//go:build gcr
// +build gcr

package repository

import (
	"errors"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

func getImageSHA(pluginToolConfig map[string]interface{}) error {
	// TODO: implement
	return nil
}

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	// TODO: implement
	return "", nil
}

func GetImageAndShaFromDownload(driverConfig *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	return errors.New("Not defined")
}
