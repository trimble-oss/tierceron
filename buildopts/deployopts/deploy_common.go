package deployopts

import (
	"fmt"
	"strings"
)

func InitSupportedDeployers(deployments []string) []string {
	return nil
}

func GetDecodedDeployerId(deployerCode string) (string, bool) {
	deployerIdParts := strings.Split(deployerCode, ".")
	if len(deployerIdParts) != 2 {
		return "", false
	}
	return fmt.Sprintf("%s.%s", deployerIdParts[0], deployerIdParts[1]), true
}

func GetEncodedDeployerId(deployment string, env string) (string, bool) {
	return fmt.Sprintf("%s.%s", deployment, env), true
}
