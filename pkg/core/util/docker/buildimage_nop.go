//go:build !docker

package docker

import eUtils "github.com/trimble-oss/tierceron/pkg/utils"

func BuildDockerImage(driverConfig *eUtils.DriverConfig, dockerfilePath, imageName string) error {
	return nil
}
