package ft

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// A ServiceDiscoverer is used to discover services on a network. The services are on some set of
// multicast addresses and a particular. The ServiceDiscoverer will broadcast on that address/port
// combination sending a user defined payload. It will then collect the responses and make them
// available.
type ServiceDiscoverer struct {
	// Port the services we are trying to discover are listening on
	Port int

	// The multicast addresses to send our broadcasts (multicasts) out on
	MulticastAddress string

	// How long to wait before sending out another broadcast
	BroadcastDelay time.Duration

	// Maximum services that listen for, -1 means there is no limit.
	MaxServices int

	// How long to spend searching for services.
	TimeLimit time.Duration

	// By default we use UDP on IPV4. If this flag is true then we use UDP on IPV6.
	UseIPV6 bool

	// This is the context sent in to the api calls then can be used to cancel the functions
	// before TimeLimit or MaxServices is reached.
	ctx context.Context

	// The payload to broadcast
	payload []byte
}

// A Service represents an address that responded to the ServiceDiscoverer broadcast.
type Service struct {
	// Address is the address of the responding service
	Address string

	// PayloadResponse is the response the service gave to our payload broadcast.
	PayloadResponse []byte
}

// FindServices will search for services on the network by broadcasting payload to the port and
// multicast address in the ServiceDiscoverer. When finished it will return the list of services
// that responded. FindServices is a synchronous call, but it starts a go routine to listen for
// responses to its payload broadcast. The context (ctx) passed in can be used to stop the broadcast
// loop and shutdown the background listener. These will automatically be cleaned up when FindServices
// terminates normally.
func (s *ServiceDiscoverer) FindServices(ctx context.Context, payload []byte) ([]Service, error) {
	s.ctx = ctx
	s.payload = payload
	addr := net.JoinHostPort(s.MulticastAddress, fmt.Sprintf("%d", s.Port))

	conn, err := net.ListenPacket(s.getUDPProtocolVersion(), addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	packetConn := s.createPacketConn(conn)

	networkInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	s.setupInterfaces(packetConn, networkInterfaces)

	return s.broadcastAndCollect(packetConn, networkInterfaces)
}

// createPacketConn creates an instance of net.PacketConn that have the same interface whether they are using
// ipv4 or ipv6.
func (s *ServiceDiscoverer) createPacketConn(conn net.PacketConn) NetPacketConn {
	if s.UseIPV6 {
		return PacketConn6{PacketConn: ipv6.NewPacketConn(conn)}
	}

	return PacketConn4{PacketConn: ipv4.NewPacketConn(conn)}
}

// setupInterfaces sets the multicast address on each of the host interfaces.
func (s *ServiceDiscoverer) setupInterfaces(packetConn NetPacketConn, networkInterfaces []net.Interface) {
	group := net.ParseIP(s.MulticastAddress)
	for _, ni := range networkInterfaces {
		_ = packetConn.JoinGroup(&ni, &net.UDPAddr{IP: group, Port: s.Port})
	}
}

// broadcastAndCollect will start up a background go routine to collect responses to its broadcasts. It will then
// enter a loop sending out broadcasts of payload (ServiceDiscoverer payload) on the multicast address and port.
func (s *ServiceDiscoverer) broadcastAndCollect(packetConn NetPacketConn, interfaces []net.Interface) ([]Service, error) {
	// Create an context so we can tell the listener go routine to stop.
	ctx, cancelCollection := context.WithCancel(context.Background())
	serviceCollector, err := s.startResponseCollectorInBackground(ctx, interfaces)
	if err != nil {
		return nil, err
	}

	udpAddr := &net.UDPAddr{IP: net.ParseIP(s.MulticastAddress), Port: s.Port}
	startingTime := time.Now()

	// Loop sending out broadcasts. The serviceCollector will collec the responses to the broadcasts.
BroadcastLoop:
	for {
		// Send broadcast
		broadcast(packetConn, s.payload, interfaces, udpAddr)

		// Stop if the user defined duration for finding services has been reached
		if discoveryDurationReached(startingTime, s.TimeLimit) {
			break
		}

		// Stop if the listener go routine has collected the user defined maximum number of
		// service responses.
		if serviceCollector.maxServicesFound {
			break
		}

		select {
		case <-s.ctx.Done():
			// Did the call decide to stop us early?
			break BroadcastLoop

		case <-time.After(s.BroadcastDelay): // Wait for the delay time before sending out another broadcast
		}
	}

	// Tell the service collector to shutdown
	cancelCollection()

	// TODO: Add a semaphore to wait on for the listen thread to mark itself as done.

	return serviceCollector.toServicesList(), nil
}

func (s *ServiceDiscoverer) startResponseCollectorInBackground(ctx context.Context, interfaces []net.Interface) (*serviceCollector, error) {
	// The service collector will collect all the responses to the broadcast.
	serviceCollector := &serviceCollector{
		sd:        s,
		responses: make(map[string][]byte),
	}

	// Make sure that we can actually create the network connection that the listener go routine
	// will listen on.
	conn, err := serviceCollector.createConnection(interfaces)
	if err != nil {
		return nil, err
	}

	// Start up a listener that will collect the responses from services that respond to the broadcast
	go serviceCollector.listenForAndCollectResponses(conn, ctx)

	return serviceCollector, nil
}

func broadcast(packetConn NetPacketConn, payload []byte, ifaces []net.Interface, dst net.Addr) {
	for i := range ifaces {
		if err := packetConn.SetMulticastInterface(&ifaces[i]); err != nil {
			// log error
			continue
		}

		_ = packetConn.SetMulticastTTL(2)
		if _, err := packetConn.WriteTo(payload, dst); err != nil {
			// log error
		}
	}
}

func discoveryDurationReached(startingTime time.Time, durationLimit time.Duration) bool {
	if durationLimit < 0 {
		return false
	}

	return time.Since(startingTime) > durationLimit
}

func (s *ServiceDiscoverer) getUDPProtocolVersion() string {
	if s.UseIPV6 {
		return "udp6"
	}

	return "udp4"
}

func (s *ServiceDiscoverer) RegisterAndListen() error {
	return nil
}
