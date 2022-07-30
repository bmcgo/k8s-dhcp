package dhcp

import (
	"context"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"net"
	"strconv"
	"sync"
	"time"
)

type CallbackSaveLeases func([]Response) error

type interfaceName string
type SubnetAddrPrefix string

type Host struct {
	Subnet         string
	MAC            string
	IP             net.IP
	Gateway        net.IP
	ServerHostName string
	BootFileName   string
	DNS            []string
	Options        []Option
	LeaseTime      int
	HostName       string
}

type Lease struct {
	Subnet         SubnetAddrPrefix
	MAC            string
	IP             net.IP
	NetMask        string
	Gateway        net.IP
	ServerHostName string
	BootFileName   string
	DNS            []string
	Options        []Option
	LeaseTime      int
	HostName       string
	ServerId       net.IP

	LastUpdate time.Time
	AckSent    bool
}

type Subnet struct {
	Subnet         SubnetAddrPrefix
	RangeFrom      string
	RangeTo        string
	Gateway        string
	DNS            []string
	Options        []Option
	LeaseTime      int
	ServerHostName string
	BootFileName   string

	iPFrom     IPv4
	iPTo       IPv4
	ipNet      net.IPNet
	currentIP  IPv4
	leaseCache map[string]*Lease
	netMask    string

	serverIPAddress net.IP
	leaseCacheMutex *sync.Mutex
}

type Server struct {
	listeners map[string]*RequestProcessor
	subnets   map[SubnetAddrPrefix]*Subnet

	localIpAddresses map[interfaceName][]net.IP
	serverIds        map[string]bool

	subnetMutex *sync.Mutex
	listenMutex *sync.Mutex

	callbackSaveLeases CallbackSaveLeases
	socketFactory      SocketFactory

	context context.Context
	log     RLogger
}

type Listen struct {
	Name      string
	Interface string
	Addr      string
}

type Option struct {
	ID    uint8
	Type  string
	Value string
}

func (l *Listen) ToString() string {
	ifname := "*"
	laddr := "*"
	if l.Interface != "" {
		ifname = l.Interface
	}
	if l.Addr != "" {
		laddr = l.Addr
	}
	return fmt.Sprintf("%s:%s", ifname, laddr)
}

type Response struct {
	Request  Request
	Response dhcpv4.DHCPv4
	Lease    *Lease
}

type Request struct {
	*dhcpv4.DHCPv4
	Src           net.Addr
	InterfaceName interfaceName
	Dst           net.IP

	socket Socket
}

func (s *Request) ToString() string {
	opts := ""
	for key, opt := range s.Options {
		opts = fmt.Sprintf("%d:%s %s", key, strconv.QuoteToASCII(string(opt)), opts)
	}
	return fmt.Sprintf("%s <%s>", s.String(), opts)
}
