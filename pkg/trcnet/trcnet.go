package trcnet

import (
	"fmt"
	"net"
	"strings"
)

func NetIpAddr(isValidIpFn func(string) (bool, error)) (string, error) {

	var lastErr error
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if strings.HasPrefix(iface.Name, "eth") {
			addrs, err := iface.Addrs()
			if err != nil {
				fmt.Println("Error getting addresses for", iface.Name, ":", err)
				continue
			}

			for _, address := range addrs {
				// Check if address belongs to eth0
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						if ok, err := isValidIpFn(ipnet.IP.String()); ok {
							return ipnet.IP.String(), nil
						} else {
							if err != nil {
								lastErr = err
							}
							continue
						}
					}
				}
			}
		}
	}
	return "", lastErr
}
