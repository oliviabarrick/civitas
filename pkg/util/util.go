package util

import (
	"net"
)

func GetIPForInterface(iface string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var ipStr string

	for _, i := range ifaces {
		if i.Name != iface {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip.To4() == nil {
				continue
			}

			ipStr = ip.String()
		}
	}

	return ipStr, nil
}
