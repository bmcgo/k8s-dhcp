package dhcp

import (
	"context"
	"errors"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"log"
	"net"
	"sync"
	"time"
)

const mirantisEntID = 45176

type LocalIPAddresses map[interfaceName][]net.IP

type ServerConfig struct {
	SocketFactory        SocketFactory
	LocalAddressesGetter func() (LocalIPAddresses, error)
	Logger               RLogger
	CallbackSaveLeases   CallbackSaveLeases
	Context              context.Context
}

func NewServer(c ServerConfig) (*Server, error) {
	var err error
	server := &Server{
		listeners: map[string]*RequestProcessor{},
		subnets:   map[SubnetAddrPrefix]*Subnet{},
		context:   c.Context,
	}
	if c.SocketFactory == nil {
		c.SocketFactory = NewUDPSocket
	}
	if c.LocalAddressesGetter == nil {
		c.LocalAddressesGetter = GetLocalAddresses
	}
	if c.CallbackSaveLeases == nil {
		return nil, errors.New("CallbackSaveLeases is mandatory")
	}

	if c.Logger == nil {
		log.Fatal("no logger set")
	}
	server.log = c.Logger.WithName("dhcp.server")
	server.socketFactory = c.SocketFactory
	server.subnetMutex = &sync.Mutex{}
	server.listenMutex = &sync.Mutex{}
	server.callbackSaveLeases = c.CallbackSaveLeases
	server.localIpAddresses, err = c.LocalAddressesGetter()
	server.serverIds = map[string]bool{}
	for _, lIPs := range server.localIpAddresses {
		for _, lIP := range lIPs {
			server.serverIds[lIP.String()] = true
		}
	}
	return server, err
}

func (s *Server) AddListen(listen Listen) error {
	var requestProcessor *RequestProcessor
	var err error
	var ok bool

	requestProcessor, ok = s.listeners[listen.Name]
	if ok {
		return fmt.Errorf("requestProcessor %v already exists", listen)
	}

	s.log.Infof("Listening %s", listen.ToString())

	requestProcessor, err = NewRequestProcessor(listen,
		s.socketFactory,
		s.callbackSaveLeases,
		s,
		s.log)

	if err != nil {
		return err
	}

	s.listeners[listen.Name] = requestProcessor
	go func() {
		err = requestProcessor.Serve()
		s.log.Infof("Exit")
		//TODO: panic if exited unexpectedly
	}()
	return nil
}

func (s *Server) AddSubnet(subnet Subnet) error {
	err := InitializeSubnet(&subnet, s.localIpAddresses)
	if err != nil {
		return err
	}

	s.subnetMutex.Lock()
	defer s.subnetMutex.Unlock()
	for _, sn := range s.subnets {
		if isOverlap(sn, &subnet) {
			return fmt.Errorf("overlapping subnets: %s, %s", sn.Subnet, subnet.Subnet)
		}
	}
	s.subnets[subnet.Subnet] = &subnet
	return nil
}

func (s *Server) DeleteSubnet(subnet SubnetAddrPrefix) error {
	s.subnetMutex.Lock()
	defer s.subnetMutex.Unlock()
	_, ok := s.subnets[subnet]
	if !ok {
		return fmt.Errorf("subnet %s not found", subnet)
	}
	delete(s.subnets, subnet)
	s.log.Infof("Deleted subnet %s", subnet)
	return nil
}

func (s *Server) AddLease(lease *Lease) error {
	s.subnetMutex.Lock()
	defer s.subnetMutex.Unlock()
	sn, ok := s.subnets[lease.Subnet]
	if !ok {
		return fmt.Errorf("can't find subnet for lease: %v", lease)
	}
	sn.AddLease(lease)
	return nil
}

func (s *Server) DeleteLease(lease *Lease) error {
	subnet := s.getSubnetForIp(lease.IP)
	if subnet == nil {
		return fmt.Errorf("subnet for lease %v not found", lease)
	}
	return subnet.DeleteLease(lease)
}

func (s *Server) AddHost(host Host) error {
	s.subnetMutex.Lock()
	defer s.subnetMutex.Unlock()
	for _, sn := range s.subnets {
		if sn.ipNet.Contains(host.IP) {
			sn.AddHost(host)
			return nil
		}
	}
	return fmt.Errorf("can't find subnet for host: %s (%s)", host.IP, host.MAC)
}

func (s *Server) DeleteHost(host Host) error {
	s.subnetMutex.Lock()
	defer s.subnetMutex.Unlock()
	for _, sn := range s.subnets {
		if sn.ipNet.Contains(host.IP) {
			sn.DeleteHost(host)
			return nil
		}
	}
	return fmt.Errorf("host not found: %v", host)
}

func (s *Server) DeleteListen(name string) error {
	s.listenMutex.Lock()
	defer s.listenMutex.Unlock()
	listen, ok := s.listeners[name]
	if !ok {
		return fmt.Errorf("unknown listen %s", name)
	}
	listen.Close()
	delete(s.listeners, name)
	return nil
}

func (s *Server) Close() {
	for _, l := range s.listeners {
		l.Close()
	}
}

func (s *Server) getSubnetForIp(ip net.IP) *Subnet {
	s.subnetMutex.Lock()
	defer s.subnetMutex.Unlock()
	var sn *Subnet
	for _, sn = range s.subnets {
		if sn.Contains(ip) {
			return sn
		}
	}
	return nil
}

func (s *Server) getSubnet(req Request) *Subnet {
	var sn *Subnet
	var ip net.IP
	if req.GatewayIPAddr == nil || req.GatewayIPAddr.Equal(net.IPv4zero) {
		s.log.Debugf("Handling broadcast request on %s", req.InterfaceName)
		for _, ip = range s.localIpAddresses[req.InterfaceName] {
			s.log.Debugf("Checking listen address %s", ip.String())
			sn = s.getSubnetForIp(ip)
			if sn != nil {
				s.log.Debugf("Found subnet: %s", sn.Subnet)
				return sn
			}
		}
	} else {
		s.log.Debugf("Handling unicast request")
		return s.getSubnetForIp(req.GatewayIPAddr)
	}
	return nil
}

func (s *Server) GetResponse(req Request) (Response, error) {
	var (
		resp     *dhcpv4.DHCPv4
		lease    *Lease
		response Response
		err      error
	)
	if req.ServerIdentifier() != nil && !req.ServerIdentifier().Equal(net.IPv4zero) {
		if _, ok := s.serverIds[req.ServerIdentifier().String()]; !ok {
			return response, fmt.Errorf("request for unknown server id: %s", req.ServerIdentifier().String())
		}
	}
	sn := s.getSubnet(req)
	if sn == nil {
		return response, fmt.Errorf("unknown subnet %s %s %s", req.Src, req.GatewayIPAddr, req.InterfaceName)
	}
	switch req.MessageType() {
	case dhcpv4.MessageTypeDiscover, dhcpv4.MessageTypeRequest:
		resp, lease, err = s.getResponse(req, sn)
		if err != nil {
			return response, err
		}
	default:
		return response, fmt.Errorf("unknown dhcp packet type %s", req.MessageType())
	}
	lease.ServerId = sn.serverIPAddress
	response.Response = *resp
	response.Lease = lease
	response.Request = req

	return response, nil
}

func (s *Server) GetLease(subnet SubnetAddrPrefix, mac string) *Lease {
	sn, ok := s.subnets[subnet]
	if !ok {
		return nil
	}
	return sn.leaseCache[mac]
}

func (s *Server) getResponse(req Request, subnet *Subnet) (*dhcpv4.DHCPv4, *Lease, error) {
	lease := subnet.GetLeaseForRequest(req.DHCPv4)
	if lease == nil {
		//TODO: return NAK
		return nil, lease, fmt.Errorf("nil lease")
	}

	resp, err := dhcpv4.NewReplyFromRequest(req.DHCPv4)
	if err != nil {
		s.log.Errorf(err, "failed to construct response for request %s", req)
		return resp, lease, err
	}
	if resp == nil {
		return resp, lease, errors.New("failed to construct response")
	}

	resp.YourIPAddr = lease.IP
	resp.ServerIPAddr = subnet.serverIPAddress
	lease.ServerId = subnet.serverIPAddress

	for _, opt := range lease.Options {
		var value dhcpv4.OptionValue
		code := opt.ID
		switch opt.Type {
		case "string":
			value = dhcpv4.String(opt.Value)
		default:
			s.log.Infof("unknown option type %s in subnet: %v", opt.Type, subnet)
			return nil, nil, fmt.Errorf("unknown option type %s", opt.Type)
		}
		resp.UpdateOption(dhcpv4.Option{Code: dhcpv4.GenericOptionCode(code), Value: value})
		if code == 66 {
			//iPXE wont boot if not set server ip addr to option 66 value
			//resp.ServerIPAddr = net.ParseIP(value.String()) //TODO
		}
	}
	resp.BootFileName = lease.BootFileName
	resp.ServerHostName = lease.ServerHostName
	resp.UpdateOption(dhcpv4.OptSubnetMask(net.IPMask(net.ParseIP(lease.NetMask).To4())))
	resp.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Duration(lease.LeaseTime) * time.Second))
	resp.UpdateOption(dhcpv4.Option{Code: dhcpv4.GenericOptionCode(3), Value: dhcpv4.IP(lease.Gateway)})
	//resp.UpdateOption(dhcpv4.Option{Code: dhcpv4.GenericOptionCode(28), Value: dhcpv4.IP{10, 12, 1, 255}}) //broadcast
	dnsServers := make([]net.IP, 0)
	for _, dns := range lease.DNS {
		dnsServers = append(dnsServers, net.ParseIP(dns).To4())
	}
	resp.UpdateOption(dhcpv4.OptDNS(dnsServers...))
	resp.UpdateOption(dhcpv4.Option{Code: dhcpv4.GenericOptionCode(54), Value: dhcpv4.IP(lease.ServerId)})
	//resp.UpdateOption(dhcpv4.OptVIVC(dhcpv4.VIVCIdentifier{EntID: mirantisEntID, Data: []byte("fo\x11obar")}))
	resp.UpdateOption(dhcpv4.Option{Code: dhcpv4.GenericOptionCode(43), Value: &dhcpv4.OptionGeneric{Data: []byte("123")}})

	switch req.MessageType() {
	case dhcpv4.MessageTypeRequest:
		resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	case dhcpv4.MessageTypeDiscover:
		resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	default:
		s.log.Infof("Unknown request type: %s", req.MessageType().String())
		return nil, nil, err
	}

	return resp, lease, err
}
