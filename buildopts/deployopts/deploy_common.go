//go:build !tc
// +build !tc

package deployopts

//	"time"

func InitSupportedDeployers() {

}

func GetDecodedDeployerId(sessionId string) (string, error) {
	return "", nil
}

func GetEncodedDeployerId(deployment string, env string) (string, bool) {
	return "", false
}
