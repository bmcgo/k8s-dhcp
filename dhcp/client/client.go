package main

import (
	"flag"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"log"
	"math/rand"
	"net"
	"syscall"
	"time"
)

func main() {

	timeOut := 15
	numClients := 0
	ifName := "br0"
	startMac := "00:01:00:00:00:00"
	flag.IntVar(&timeOut, "timeout", 15, "Timeout")
	flag.IntVar(&numClients, "num-clients", 1, "Number of clients")
	flag.StringVar(&ifName, "interface", "br0", "Interface name")
	flag.StringVar(&startMac, "start-mac", "00:01:00:00:00:00", "Start mac address")
	flag.Parse()

	donech := make(chan bool)
	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		log.Fatal(err)
	}

	cm, err := net.ParseMAC(startMac)
	if err != nil {
		log.Fatal(err)
	}
	clients := map[string]*Client{}
	for i := 0; i < numClients; i++ {
		incMac(cm)
		client := &Client{
			Offers: []dhcpv4.DHCPv4{},
			MAC:    []byte{cm[0], cm[1], cm[2], cm[3], cm[4], cm[5]},
		}
		clients[cm.String()] = client
	}

	ch, _, err := getEthernetReceiver(iface)
	if err != nil {
		log.Fatal(err)
	}

	go GetDHCPAddrs(clients, ch, donech)
	for _, c := range clients {
		go c.SendDiscovers(iface)
	}

	success := 0
	select {
	case <-donech:
	case <-time.After(time.Second * time.Duration(timeOut)):
	}
	for _, c := range clients {
		if len(c.Offers) == 0 {
			log.Printf("no offers for: %s", c.MAC)
		} else {
			if c.Ack == nil {
				log.Printf("no ack for: %s", c.MAC)
			} else {
				success++
			}
		}
	}
	log.Printf("success: %d/%d", success, len(clients))
}

func incMac(addr net.HardwareAddr) {
	addr[5]++
	if addr[5] == 0 {
		addr[4]++
		if addr[4] == 0 {
			addr[3]++
			if addr[3] == 0 {
				addr[2]++
				if addr[2] == 0 {
					addr[1]++
					if addr[1] == 0 {
						addr[0]++
					}
				}
			}
		}
	}
}

type Client struct {
	MAC    net.HardwareAddr
	Offers []dhcpv4.DHCPv4
	Ack    *dhcpv4.DHCPv4
}

func (c *Client) ProcessResponse(resp dhcpv4.DHCPv4) error {
	//log.Printf("processing response for: %s", c.MAC)
	if resp.OpCode != dhcpv4.OpcodeBootReply {
		return fmt.Errorf("invalid packet opcode:  %s", resp.String())
	}
	switch resp.MessageType() {
	case dhcpv4.MessageTypeOffer:
		c.Offers = append(c.Offers, resp)
	case dhcpv4.MessageTypeAck:
		if c.Ack != nil {
			//log.Printf("extra ack reveived for client: %s", resp.String())
		} else {
			c.Ack = &resp
		}
	default:
		return fmt.Errorf("unknown dhcp message type: %s", resp.String())
	}
	return nil
}

func (c *Client) SendDiscovers(iface *net.Interface) {
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(800)))
	d, err := dhcpv4.NewDiscovery(c.MAC)
	if err != nil {
		log.Printf("failed to create discovery packet: %s", err)
	}
	for i := 0; i < 13; i++ {
		//log.Printf("sending discover: %s", c.MAC)
		err := sendEthernet(
			*iface,
			c.MAC,
			net.IPv4zero,
			dhcpv4.ClientPort,
			net.HardwareAddr{255, 255, 255, 255, 255, 255},
			net.IPv4bcast,
			dhcpv4.ServerPort,
			d,
		)
		if err != nil {
			log.Printf("error sending discover: %s", err)
		}
		time.Sleep(time.Second * 1)
		if i > 2 {
			if len(c.Offers) > 0 {
				break
			}
		}
	}
	for {
		c.sendRequest(iface)
		time.Sleep(time.Millisecond * 300)
		if c.Ack != nil {
			break
		}
	}
}

func (c *Client) sendRequest(iface *net.Interface) {
	if len(c.Offers) < 1 {
		log.Printf("no offers received for %s", c.MAC)
		return
	}
	offer := c.Offers[len(c.Offers)-1]
	req, err := dhcpv4.NewRequestFromOffer(&offer)
	if err != nil {
		log.Printf("failed to construct request: %s", err)
	}
	err = sendEthernet(*iface,
		c.MAC,
		net.IPv4zero,
		dhcpv4.ClientPort,
		net.HardwareAddr{255, 255, 255, 255, 255, 255},
		net.IPv4bcast,
		dhcpv4.ServerPort,
		req)
	if err != nil {
		log.Printf("failed to send request: %s", err)
	}
}

func GetDHCPAddrs(clients map[string]*Client, ch <-chan []byte, donech chan<- bool) {
	for {
	beg:
		buf, more := <-ch
		if !more {
			log.Println("receiver closed")
			return
		}
		clientMAC := net.HardwareAddr{buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]}
		client, ok := clients[clientMAC.String()]
		if !ok {
			//log.Printf("unknown target mac: %s", clientMAC)
			continue
		}
		packet, err := dhcpv4.FromBytes(buf[42:])
		if err != nil {
			log.Printf("failed to parse packet: %s", err)
			continue
		}
		//log.Println(packet)
		err = client.ProcessResponse(*packet)
		if err != nil {
			log.Printf("failed to process response: %s", err)
		}
		if client.Ack != nil {
			for _, c := range clients {
				if c.Ack == nil {
					goto beg
				}
			}
			donech <- true
			return
		}
	}
}

func getEthernetReceiver(iface *net.Interface) (<-chan []byte, int, error) {
	ch := make(chan []byte, 1024)

	htons_ETH_P_ALL := 0x0300 //syscall.ETH_P_ALL = 0x3
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, htons_ETH_P_ALL)
	if err != nil {
		return nil, fd, err
	}

	syscall.SetLsfPromisc(iface.Name, true)

	sa := &syscall.SockaddrLinklayer{
		Protocol: uint16(htons_ETH_P_ALL),
		Ifindex:  iface.Index,
		Hatype:   syscall.ARPHRD_ETHER,
		Pkttype:  syscall.PACKET_OTHERHOST,
		Halen:    6,
	}
	err = syscall.Bind(fd, sa)
	if err != nil {
		return nil, fd, err
	}
	go func() {
		defer syscall.SetLsfPromisc(iface.Name, false)
		for {
			buf := make([]byte, 1500)
			n, _, err := syscall.Recvfrom(fd, buf, 0)
			if err != nil {
				log.Printf("recvfrom: %s", err)
				break
			}
			ch <- buf[:n]
		}
	}()
	return ch, fd, nil
}

func sendEthernet(
	iface net.Interface,
	srcMAC net.HardwareAddr,
	srcIP net.IP,
	srcPort layers.UDPPort,
	dstMAC net.HardwareAddr,
	dstIP net.IP,
	dstPort layers.UDPPort,
	dhcPv4 *dhcpv4.DHCPv4,
) error {

	eth := layers.Ethernet{
		EthernetType: layers.EthernetTypeIPv4,
		SrcMAC:       srcMAC,
		DstMAC:       dstMAC,
	}
	ip := layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    srcIP,
		DstIP:    dstIP,
		Protocol: layers.IPProtocolUDP,
		Flags:    layers.IPv4DontFragment,
	}
	udp := layers.UDP{
		SrcPort: srcPort,
		DstPort: dstPort,
	}

	err := udp.SetNetworkLayerForChecksum(&ip)
	if err != nil {
		return fmt.Errorf("send Ethernet: couldn't set network layer: %v", err)
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// Decode a packet
	packet := gopacket.NewPacket(dhcPv4.ToBytes(), layers.LayerTypeDHCPv4, gopacket.NoCopy)
	dhcpLayer := packet.Layer(layers.LayerTypeDHCPv4)
	dhcp, ok := dhcpLayer.(gopacket.SerializableLayer)
	if !ok {
		return fmt.Errorf("layer %s is not serializable", dhcpLayer.LayerType().String())
	}
	err = gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, dhcp)
	if err != nil {
		return fmt.Errorf("can't serialize layer: %v", err)
	}
	data := buf.Bytes()

	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, 0)
	if err != nil {
		return fmt.Errorf("send Ethernet: can't open socket: %v", err)
	}
	defer func() {
		err = syscall.Close(fd)
		if err != nil {
			log.Printf("Send Ethernet: Cannot close socket: %v", err)
		}
	}()

	err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		return fmt.Errorf("send Ethernet: can't set option for socket: %v", err)
	}

	var hwAddr [8]byte
	copy(hwAddr[0:6], dhcPv4.ClientHWAddr[0:6])
	ethAddr := syscall.SockaddrLinklayer{
		Protocol: 0,
		Ifindex:  iface.Index,
		Halen:    6,
		Addr:     hwAddr,
	}
	err = syscall.Sendto(fd, data, 0, &ethAddr)
	if err != nil {
		return fmt.Errorf("can't send frame via socket: %v", err)
	}
	//log.Printf("sent: %s", dhcPv4.String())
	return nil
}
