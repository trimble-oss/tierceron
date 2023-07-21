//go:build !gcr && !azrcr && !awsecr
// +build !gcr,!azrcr,!awsecr

package repository

import "errors"

import (
	eUtils "github.com/trimble-oss/tierceron/utils"
)

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	return "", nil
}

func GetImageAndShaFromDownload(config *eUtils.DriverConfig, pluginToolConfig map[string]interface{}) error {
	return errors.New("Not defined")
}
