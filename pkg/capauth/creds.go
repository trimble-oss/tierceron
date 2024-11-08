package capauth

import (
	"errors"
	"net"
	"strings"

	"github.com/trimble-oss/tierceron/pkg/trcnet"
)

func LoopBackAddr() string {
	return "127.0.0.1"
}

func LocalAddr(env string) (string, error) {
	localIP, err := trcnet.LocalIp()
	if err != nil {
		return "", err
	}
	addrs, hostErr := net.LookupAddr(localIP)
	if hostErr != nil {
		return "", hostErr
	}
	localHost := ""
	if len(addrs) > 0 {
		if len(addrs) > 20 {
			return "", errors.New("unsupported hosts")
		}
		for _, addr := range addrs {
			localHost = strings.TrimRight(addr, ".")
			if validErr := ValidateVhost(localHost, "", false); validErr != nil {
				localHost = ""
				continue
			} else {
				break
			}
		}
	} else {
		return "", errors.New("invalid host")
	}

	return localHost, nil
}
