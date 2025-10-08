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

func IsValidIP(ipaddr string) (bool, error) {
	addrs, hostErr := net.LookupAddr(ipaddr)
	if hostErr != nil {
		return false, nil
	}
	trimmedAddr := ""
	if len(addrs) > 0 {
		if len(addrs) > 20 {
			return false, errors.New("unsupported hosts")
		}
		for _, addr := range addrs {
			trimmedAddr = strings.TrimRight(addr, ".")
			if validErr := ValidateVhost(trimmedAddr, "", false); validErr != nil {
				trimmedAddr = ""
				continue
			} else {
				break
			}
		}
	} else {
		return false, nil
	}
	return true, nil
}

func TrcNetAddr() (string, error) {
	netIP, err := trcnet.NetIpAddr(IsValidIP)
	if err != nil {
		return "", err
	}

	return netIP, nil
}
