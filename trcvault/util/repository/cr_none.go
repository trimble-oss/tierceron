//go:build (!gcr && ignore) || (!azrcr && ignore) || !awsecr
// +build !gcr,ignore !azrcr,ignore !awsecr

package repository

import "errors"

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	return "", nil
}

func GetImageAndShaFromDownload(pluginToolConfig map[string]interface{}) error {
	return errors.New("Not defined")
}
