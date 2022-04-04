//go:build (!gcr && ignore) || (!azrcr && ignore) || !awsecr
// +build !gcr,ignore !azrcr,ignore !awsecr

package repository

// Return url to the image to be used for download.
func GetImageDownloadUrl(pluginToolConfig map[string]interface{}) (string, error) {
	return "", nil
}

func GetShaFromDownload(pluginToolConfig map[string]interface{}) {
	return
}
