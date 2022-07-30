package dhcp

import (
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"golang.org/x/net/ipv4"
	"net"
	"strconv"
	"strings"
	"sync"
)

type SocketFactory func(listenAddress string, listenInterface string, logger RLogger) (Socket, error)

type Socket interface {
	NextRequest() (*Request, error)
	//SendResp(Response) error //TODO
	SendResponse(Request, dhcpv4.DHCPv4) error
	SendBroadcast(req Request, resp dhcpv4.DHCPv4) error
	Close()
}

type UDPSocket struct {
	udpConn    *net.UDPConn
	packetConn *ipv4.PacketConn

	interfaceName string
	listenAddress string
	listenPort    int

	bcResponders     map[interfaceName]*BroadcastResponder
	bcRespondersLock sync.RWMutex

	log RLogger
}

func (s *UDPSocket) Close() {
	err := s.packetConn.Close()
	if err != nil {
		s.log.Errorf(err, "failed to close socket")
	}
}

func (s *UDPSocket) NextRequest() (*Request, error) {
	var i *net.Interface
	buf := make([]byte, 1<<16)
	n, cm, src, err := s.packetConn.ReadFrom(buf)
	if err != nil {
		return nil, err
	}
	i, err = net.InterfaceByIndex(cm.IfIndex)
	if err != nil {
		return nil, err
	}
	req := Request{
		Src:           src,
		InterfaceName: interfaceName(i.Name),
		Dst:           cm.Dst,
		socket:        s,
	}
	req.DHCPv4, err = dhcpv4.FromBytes(buf[:n])
	if err != nil {
		return nil, err
	}
	s.log.Infof(req.ToString())
	return &req, nil
}

func (s *UDPSocket) SendResponse(req Request, resp dhcpv4.DHCPv4) error {
	src, err := getSrcAddr(req.GatewayIPAddr)
	if err != nil {
		return err
	}
	s.log.Debugf("Set ServerID: %s", src)
	resp.UpdateOption(dhcpv4.Option{Code: dhcpv4.GenericOptionCode(54), Value: dhcpv4.IP(src)})
	n, err := s.packetConn.WriteTo(resp.ToBytes(), nil, req.Src)
	s.log.Infof("%d bytes sent %s -> %s", n, req.Dst, req.Src)
	return err
}

func getSrcAddr(dst net.IP) (src net.IP, err error) {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   dst,
		Port: 123,
	})
	if err != nil {
		return
	}
	defer conn.Close()
	src = conn.LocalAddr().(*net.UDPAddr).IP
	return
}

func (s *UDPSocket) SendBroadcast(req Request, resp dhcpv4.DHCPv4) error {
	var (
		ok        bool
		err       error
		responder *BroadcastResponder
	)
	s.log.Debugf("Sending broadcast response to %s", resp.ClientHWAddr)
	s.bcRespondersLock.RLock()
	responder, ok = s.bcResponders[req.InterfaceName]
	s.bcRespondersLock.RUnlock()
	if !ok {
		responder, err = RawSocketBroadcastResponderFactory(req.InterfaceName, s.log)
		if err != nil {
			return err
		}
		s.bcRespondersLock.Lock()
		s.bcResponders[req.InterfaceName] = responder
		s.bcRespondersLock.Unlock()
	}
	return responder.Send(req, resp)
}

func NewUDPSocket(listenAddress string, listenInterface string, logger RLogger) (Socket, error) {
	var port int64
	var err error

	ifname := "*"
	laddr_ := "*"

	if listenInterface != "" {
		ifname = listenInterface
	}

	if listenAddress != "" {
		laddr_ = listenAddress
	}

	udpSocket := UDPSocket{
		interfaceName:    listenInterface,
		listenAddress:    listenAddress,
		log:              logger.WithName(fmt.Sprintf("socket[%s:%s]", ifname, laddr_)),
		bcResponders:     map[interfaceName]*BroadcastResponder{},
		bcRespondersLock: sync.RWMutex{},
	}
	laddr := strings.Split(listenAddress, ":")
	if len(laddr) > 2 {
		return nil, fmt.Errorf("invalid listen address %s", listenAddress)
	}
	if len(laddr) == 2 {
		port, err = strconv.ParseInt(laddr[1], 10, 16)
		if err != nil {
			return nil, err
		}
	} else {
		port = dhcpv4.ServerPort
	}
	udpSocket.listenPort = int(port)
	addr := &net.UDPAddr{
		IP:   net.ParseIP(laddr[0]),
		Port: int(port),
	}
	udpSocket.udpConn, err = server4.NewIPv4UDPConn(listenInterface, addr)
	if err != nil {
		return nil, err
	}
	udpSocket.packetConn = ipv4.NewPacketConn(udpSocket.udpConn)
	err = udpSocket.packetConn.SetControlMessage(ipv4.FlagInterface, true)
	return &udpSocket, err
}
