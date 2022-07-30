package dhcp

import (
	"net"
	"strings"
)

// GetLocalAddresses return map
// "eth0": {"10.1.1.1", "10.2.2.2"}
func GetLocalAddresses() (LocalIPAddresses, error) {
	rv := make(map[interfaceName][]net.IP)
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifs {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			_ifs := rv[interfaceName(i.Name)]
			_addrs := strings.Split(addr.String(), "/")
			if strings.Contains(_addrs[0], ":") {
				continue
			}
			rv[interfaceName(i.Name)] = append(_ifs, net.ParseIP(_addrs[0]))
		}
	}
	return rv, nil
}

func isOverlap(s1 *Subnet, s2 *Subnet) bool {
	if s1.Subnet == s2.Subnet {
		return true
	}
	//TODO: check overlap
	return false
}

func (l Lease) IsExpired() bool {
	//TODO
	return false
}

func isAddressZero(ip net.IP) bool {
	return ip == nil || ip.Equal(net.IPv4zero)
}
