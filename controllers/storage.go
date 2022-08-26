package controllers

import (
	dhcpv1alpha1 "github.com/bmcgo/k8s-dhcp/api/v1alpha1"
	"github.com/bmcgo/k8s-dhcp/dhcp"
	"sync"
)

//ObjectsCache is temporary storage for objects with yet unknown owners,
// e.g. at startup DHCPSubnet may be loaded before DHCPServer
type ObjectsCache struct {
	knownSubnets map[dhcp.SubnetAddrPrefix]dhcp.Subnet
	unknownHosts map[dhcp.SubnetAddrPrefix][]dhcpv1alpha1.DHCPHost
	knownListens map[string]*dhcpv1alpha1.DHCPServer
	knownLeases  map[string]bool

	offersSavingLock   sync.Mutex
	lock               sync.Mutex
	ListensLock        sync.Mutex
	knownLeasesRWMutex sync.RWMutex
}

func NewObjectsCache() *ObjectsCache {
	return &ObjectsCache{
		knownSubnets:       map[dhcp.SubnetAddrPrefix]dhcp.Subnet{},
		unknownHosts:       map[dhcp.SubnetAddrPrefix][]dhcpv1alpha1.DHCPHost{},
		knownListens:       map[string]*dhcpv1alpha1.DHCPServer{},
		lock:               sync.Mutex{},
		offersSavingLock:   sync.Mutex{},
		ListensLock:        sync.Mutex{},
		knownLeasesRWMutex: sync.RWMutex{},
	}
}

func (s *ObjectsCache) AddLease(mac string) {
	s.knownLeasesRWMutex.Lock()
	defer s.knownLeasesRWMutex.Unlock()
	s.knownLeases[mac] = true
}

func (s *ObjectsCache) HasLease(mac string) bool {
	s.knownLeasesRWMutex.RLock()
	defer s.knownLeasesRWMutex.RUnlock()
	return s.knownLeases[mac]
}

func (s *ObjectsCache) AddHostIfNotKnown(host dhcpv1alpha1.DHCPHost) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	subnetName := dhcp.SubnetAddrPrefix(host.Spec.Subnet)
	_, ok := s.knownSubnets[subnetName]
	if !ok {
		if _, found := s.unknownHosts[subnetName]; !found {
			s.unknownHosts[subnetName] = []dhcpv1alpha1.DHCPHost{host}
		} else {
			s.unknownHosts[subnetName] = append(s.unknownHosts[subnetName], host)
		}
		return true
	}
	return false
}

func (s *ObjectsCache) AddSubnetIfNotKnown(subnet dhcp.Subnet) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.knownSubnets[subnet.Subnet]; ok {
		return false
	}
	s.knownSubnets[subnet.Subnet] = subnet
	return true
}

func (s *ObjectsCache) PopUnknownHosts(subnet dhcp.SubnetAddrPrefix) []dhcpv1alpha1.DHCPHost {
	s.lock.Lock()
	defer s.lock.Unlock()
	hosts := s.unknownHosts[subnet]
	delete(s.unknownHosts, subnet)
	return hosts
}
