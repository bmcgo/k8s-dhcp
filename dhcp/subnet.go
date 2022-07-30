package dhcp

import (
	"errors"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultLeaseTime = 14400 //4 hours
)

func (s *Subnet) Contains(ip net.IP) bool {
	return s.ipNet.Contains(ip)
}

func InitializeSubnet(subnet *Subnet, addrs LocalIPAddresses) error {
	var err error
	subnet.iPFrom, err = ParseIPv4(subnet.RangeFrom)
	if err != nil {
		return err
	}
	subnet.iPTo, err = ParseIPv4(subnet.RangeTo)
	if err != nil {
		return err
	}
	if subnet.iPFrom > subnet.iPTo {
		return errors.New("from > to")
	}
	subnet.leaseCache = make(map[string]*Lease)
	if subnet.LeaseTime == 0 {
		subnet.LeaseTime = defaultLeaseTime
	}
	sn := strings.Split(string(subnet.Subnet), "/")
	if len(sn) != 2 {
		return fmt.Errorf("invalid subnet %q (%v)", subnet.Subnet, subnet)
	}
	prefixLength, err := strconv.ParseInt(sn[1], 10, 8)
	if err != nil {
		return err
	}
	ipAddr := net.ParseIP(sn[0])
	ipMask := net.CIDRMask(int(prefixLength), 32)
	subnet.ipNet = net.IPNet{
		IP:   ipAddr,
		Mask: ipMask,
	}
	subnet.netMask = net.IP(ipMask).String()
	subnet.leaseCacheMutex = &sync.Mutex{}

	for _, as := range addrs {
		for _, a := range as {
			if subnet.Contains(a) {
				subnet.serverIPAddress = a
			}
		}
	}
	return nil
}

func (s *Subnet) incrementCurrentIP() {
	s.currentIP.Inc()
	if s.currentIP > s.iPTo {
		s.currentIP = s.iPFrom
	}
}

func (s *Subnet) AddLease(l *Lease) {
	ip := l.IP.String()
	old, ok := s.leaseCache[ip]
	if ok {
		delete(s.leaseCache, old.MAC)
	}
	s.leaseCache[ip] = l
	s.leaseCache[l.MAC] = l
}

func (s *Subnet) DeleteHost(host Host) {
	delete(s.leaseCache, host.MAC)
	delete(s.leaseCache, host.IP.String())
}

func (s *Subnet) AddHost(h Host) {
	s.leaseCacheMutex.Lock()
	defer s.leaseCacheMutex.Unlock()
	lease := &Lease{
		MAC:            h.MAC,
		IP:             h.IP,
		NetMask:        s.netMask,
		Gateway:        h.Gateway,
		ServerHostName: h.ServerHostName,
		BootFileName:   h.BootFileName,
		DNS:            h.DNS,
		Options:        h.Options,
		LeaseTime:      h.LeaseTime,
		HostName:       h.HostName,
	}
	s.AddLease(lease)
}

func (s *Subnet) GetLeaseForRequest(req *dhcpv4.DHCPv4) *Lease {
	s.leaseCacheMutex.Lock()
	defer s.leaseCacheMutex.Unlock()
	var (
		lease            *Lease
		oldestLease      *Lease
		ok               bool
		requestedAddress net.IP
	)
	mac := req.ClientHWAddr.String()

	//Check if lease is in cache. Make sure if requested IP matched. Return NAK otherwise
	requestedAddress = req.RequestedIPAddress()
	lease, ok = s.leaseCache[mac]
	if ok {
		if !isAddressZero(requestedAddress) && !requestedAddress.Equal(lease.IP) {
			log.Printf("requested address is not available: %s (%s)", requestedAddress, lease.IP)
			//TODO: send NAK
			return nil
		}
		return lease
	}

	//Lease is not in cache, so this is a new discovery request
	//Check if requested address is available
	if !isAddressZero(requestedAddress) {
		lease, ok = s.leaseCache[requestedAddress.String()]
		if ok {
			if lease.IsExpired() {
				lease = s.NewLease(mac, requestedAddress)
				s.AddLease(lease)
				return lease
			} else {
				log.Println("requested address is not available")
				return nil
			}
		}
		return s.NewLease(mac, requestedAddress)
	}

	//No address requested. Let's pick one from range
	if s.currentIP == 0 {
		s.currentIP = s.iPFrom
	} else {
		s.incrementCurrentIP()
	}
	expiredTime := time.Now().Add(-time.Second * time.Duration(s.LeaseTime))
	firstIp := s.currentIP
	for {
		lease, ok = s.leaseCache[s.currentIP.String()]
		if !ok {
			lease = s.NewLease(mac, net.ParseIP(s.currentIP.String()))
			s.AddLease(lease)
			return lease
		}
		if lease.LastUpdate.Before(expiredTime) {
			if oldestLease == nil {
				oldestLease = lease
			} else {
				if oldestLease.LastUpdate.After(lease.LastUpdate) {
					oldestLease = lease
				}
			}
		}
		s.incrementCurrentIP()
		if firstIp == s.currentIP {
			if oldestLease != nil {
				return oldestLease
			} else {
				log.Println("No available addresses in pool")
				//TODO: send NAK
				return nil
			}
		}
	}
}

func (s *Subnet) NewLease(mac string, ip net.IP) *Lease {
	return &Lease{
		Subnet:         s.Subnet,
		MAC:            mac,
		IP:             ip,
		LastUpdate:     time.Now(),
		Options:        s.Options,
		NetMask:        s.netMask,
		Gateway:        net.ParseIP(s.Gateway),
		DNS:            s.DNS,
		LeaseTime:      s.LeaseTime,
		BootFileName:   s.BootFileName,
		ServerHostName: s.ServerHostName,
		ServerId:       s.serverIPAddress,
	}
}

func (s *Subnet) DeleteLease(lease *Lease) error {
	s.leaseCacheMutex.Lock()
	defer s.leaseCacheMutex.Unlock()
	delete(s.leaseCache, lease.MAC)
	delete(s.leaseCache, lease.IP.String())
	return nil
}
