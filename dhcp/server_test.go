package dhcp

import (
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/stretchr/testify/require"
	"log"
	"net"
	"testing"
)

func mockSaveLeasesCallback(resps []Response) error {
	return nil
}

type MockSocket struct {
	listenAddress  string
	interfaceName  string
	requestChan    chan Request
	responseChan   chan dhcpv4.DHCPv4
	currentRequest int
}

func (s *MockSocket) NextRequest() (*Request, error) {
	req := <-s.requestChan
	return &req, nil
}

func (s *MockSocket) SendResponse(req Request, resp dhcpv4.DHCPv4) error {
	s.responseChan <- resp
	return nil
}

func (s *MockSocket) SendBroadcast(req Request, resp dhcpv4.DHCPv4) error {
	s.responseChan <- resp
	return nil
}

func (s *MockSocket) Close() {
	return
}

type ReqResp struct {
	req  Request
	resp *dhcpv4.DHCPv4
}

func mockGetLocalAddresses() (LocalIPAddresses, error) {
	la := map[interfaceName][]net.IP{
		"br0": {
			net.ParseIP("10.1.1.1"),
			net.ParseIP("10.2.1.1"),
		},
		"br1": {
			net.ParseIP("10.3.1.1"),
		},
	}
	return la, nil
}

type mockSocketFactory struct {
	mockSocket   MockSocket
	requestChan  chan Request
	responseChan chan dhcpv4.DHCPv4
}

func (f *mockSocketFactory) Factory(listenAddress string, interfaceName string, _ RLogger) (Socket, error) {
	f.mockSocket = MockSocket{listenAddress: listenAddress,
		interfaceName: interfaceName,
		requestChan:   f.requestChan,
		responseChan:  f.responseChan,
	}
	return &f.mockSocket, nil
}

func TestNewServer(t *testing.T) {
	requestChan := make(chan Request, 16)
	responseChan := make(chan dhcpv4.DHCPv4, 16)
	socketFactory := mockSocketFactory{requestChan: requestChan, responseChan: responseChan}

	m, err := NewServer(ServerConfig{
		CallbackSaveLeases:   mockSaveLeasesCallback,
		SocketFactory:        socketFactory.Factory,
		LocalAddressesGetter: mockGetLocalAddresses,
		Logger:               &GenericLogger{},
	})
	require.NoError(t, err)
	require.NotNil(t, m)

	err = m.AddListen(Listen{
		Interface: "br0",
		Addr:      "0.0.0.0",
	})
	require.NoError(t, err)

	sn1 := Subnet{
		Subnet:    "192.168.10.0/24",
		RangeFrom: "192.168.10.100",
		RangeTo:   "192.168.10.200",
		Gateway:   "192.168.10.1",
		DNS:       []string{"1.1.1.1", "2.2.2.2"},
		LeaseTime: 3600,
		Options:   nil,
	}
	err = m.AddSubnet(sn1)
	require.NoError(t, err)

	sn2 := Subnet{
		Subnet:    "10.3.1.0/24",
		RangeFrom: "10.3.1.10",
		RangeTo:   "10.3.1.13",
		Gateway:   "10.3.1.254",
		DNS:       []string{"1.1.1.1", "2.2.2.2"},
		LeaseTime: 3600,
		Options:   nil,
	}
	err = m.AddSubnet(sn2)
	require.NoError(t, err)

	dr := &dhcpv4.DHCPv4{
		OpCode:        dhcpv4.OpcodeBootRequest,
		HWType:        iana.HWTypeEthernet,
		TransactionID: dhcpv4.TransactionID{1, 2, 3, 4},
		//GatewayIPAddr: net.ParseIP("192.168.10.1"),
		ClientHWAddr: net.HardwareAddr{1, 2, 3, 4, 5, 6},
	}
	dr.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
	log.Println(dr)
	requestChan <- Request{
		DHCPv4:        dr,
		Src:           nil,
		InterfaceName: "br1",
		socket:        &socketFactory.mockSocket,
	}
	resp := <-responseChan
	assertEqual(t, resp.YourIPAddr.String(), "10.3.1.10")
	m.Close()
}
