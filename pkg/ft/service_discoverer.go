package ft

import (
	"context"
	"fmt"
	"net"
	"time"
)

// A ServiceDiscoverer is used to discover services on a network. The services are on some set of
// multicast addresses and a particular port. The ServiceDiscoverer will broadcast on that address/port
// combination sending a user defined payload. It will then collect the responses and make them
// available.
type ServiceDiscoverer struct {
	// Port the services we are trying to discover are listening on. Defaults to 9999.
	Port int

	// The multicast addresses to send our broadcasts (multicasts) out on. Defaults
	// to 239.255.255.250 for ipv4 and ff02::c for ipv6.
	MulticastAddress string

	// How long to wait before sending out another broadcast. Defaults to 1 second.
	BroadcastDelay time.Duration

	// Maximum services to collect, -1 means there is no limit. Defaults to -1 (unlimited).
	MaxServices int

	// How long to spend searching for services. Defaults to 10 seconds.
	TimeLimit time.Duration

	// By default we use UDP on IPV4. If this flag is true then we use UDP on IPV6. Defaults to false.
	UseIPV6 bool

	// Return services found on the local host. Defaults to false.
	AllowLocal bool

	// The OnServiceFoundFunc will be called when a service entry is found. The function returns a
	// boolean value. If true is returned then collection will stop (presumably the desired service
	// was found. If false is returned then collection will continue. Defaults to a func that always
	// returns false.
	OnServiceFoundFunc OnServiceFoundFunc

	// ****** Internal state ******

	// This is the context sent in to the api calls that can be used to cancel the functions
	// before TimeLimit or MaxServices is reached.
	ctx context.Context

	// The payload to broadcast
	payload []byte

	// The network interfaces to the host that we will broadcast to or listen on
	interfaces []net.Interface

	// Network packet connector interface - mediates between ipv6 and ipv4 network interfaces
	packetConn NetPacketConn
}

// OnServiceFoundFunc reprsents the function to call when a new service is discovered. The function
// should return true if collection should stop, and false otherwise.
type OnServiceFoundFunc func(service *Service) bool

var DefaultOnServiceFoundFunc OnServiceFoundFunc = func(service *Service) bool {
	return false
}

// A Service represents an address that responded to the ServiceDiscoverer broadcast.
type Service struct {
	// Address is the address of the responding service
	Address string

	// PayloadResponse is the response the service gave to our payload broadcast.
	PayloadResponse []byte
}

// FindServices will search for services on the network by listening for broadcasts to the port and
// multicast address in the ServiceDiscoverer. When finished it will return the list of services
// that responded. FindServices is a synchronous call, but it starts a go routine to listen for
// responses to its payload broadcast. The context (ctx) passed in can be used to stop the broadcast
// loop and shutdown the background listener. These will automatically be cleaned up when FindServices
// terminates.
func (s *ServiceDiscoverer) FindServices(ctx context.Context, payload []byte) ([]Service, error) {
	if err := s.finishServiceDiscovererSetup(ctx, payload); err != nil {
		return nil, err
	}

	defer s.packetConn.Close()

	sdLoop := &serviceDiscovererLoop{sd: s}
	return sdLoop.collectLoop()
}

// BroadcastService will broadcast payload on the network but will not discover services. BroadcastService is
// a synchronous call. At the moment it does start a background listener that it ignores.
func (s *ServiceDiscoverer) BroadcastService(ctx context.Context, payload []byte) error {
	if err := s.finishServiceDiscovererSetup(ctx, payload); err != nil {
		return err
	}

	defer s.packetConn.Close()

	// Address to broadcast on
	broadcastAddr := &net.UDPAddr{IP: net.ParseIP(s.MulticastAddress), Port: s.Port}

	sdLoop := &serviceDiscovererLoop{sd: s}
	sdLoop.broadcastLoop(broadcastAddr)

	return nil
}

// BroadcastAndFindServices acts as both a service broadcaster and a service finder. See BroadcastService and
// FindServices for details.
func (s *ServiceDiscoverer) BroadcastAndFindServices(ctx context.Context, payload []byte) ([]Service, error) {
	if err := s.finishServiceDiscovererSetup(ctx, payload); err != nil {
		return nil, err
	}

	defer s.packetConn.Close()

	// Address to broadcast on
	broadcastAddr := &net.UDPAddr{IP: net.ParseIP(s.MulticastAddress), Port: s.Port}

	sdLoop := &serviceDiscovererLoop{sd: s}
	return sdLoop.broadcastAnCollectLoop(broadcastAddr)
}

// finishServiceDiscovererSetup finishes the book keeping around setting up a ServiceDiscoverer including
// setting the default values for unset flags, setting multicast and creating a connection.
func (s *ServiceDiscoverer) finishServiceDiscovererSetup(ctx context.Context, payload []byte) error {
	// Carry context and payload into broadcast
	s.ctx = ctx
	s.payload = payload

	s.setDefaultValuesForServiceDiscoverer()

	// Find all the host network interfaces
	networkInterfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	s.interfaces = networkInterfaces

	// Create the network connection
	addr := net.JoinHostPort(s.MulticastAddress, fmt.Sprintf("%d", s.Port))
	conn, err := net.ListenPacket(getUDPProtocolVersion(s.UseIPV6), addr)
	if err != nil {
		return err
	}

	// Transform connection so it is ipv4/ipv6 independent and use that
	s.packetConn = createNetPacketConn(conn, s.UseIPV6)

	// Now that we are ipv4/ipv6 independent setup the network interfaces
	setupInterfaces(s.packetConn, s.interfaces, s.MulticastAddress, s.Port)

	return nil
}

// setDefaultValuesForServiceDiscoverer sets a default value in ServiceDiscoverer for any value that the user
// did not set.
func (s *ServiceDiscoverer) setDefaultValuesForServiceDiscoverer() {
	if s.MaxServices == 0 {
		// MaxServices not set, default to unlimited (-1)
		s.MaxServices = -1
	}

	if s.Port == 0 {
		s.Port = 9999
	}

	if s.MulticastAddress == "" {
		s.MulticastAddress = "239.255.255.250"
		if s.UseIPV6 {
			s.MulticastAddress = "ff02::c"
		}
	}

	if s.TimeLimit == 0 {
		s.TimeLimit = 10 * time.Second
	}

	if s.BroadcastDelay == 0 {
		s.BroadcastDelay = 1 * time.Second
	}

	if s.OnServiceFoundFunc == nil {
		s.OnServiceFoundFunc = DefaultOnServiceFoundFunc
	}
}
