//go:build (!gcr && ignore) || (!azrcr && ignore) || !awsecr
// +build !gcr,ignore !azrcr,ignore !awsecr

package repository

func getImageSHA(pluginToolConfig map[string]interface{}) error {
	// TODO: implement
	return nil
}

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	// TODO: implement
	return "", nil
}
