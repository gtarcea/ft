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
	// Port the services we are trying to discover are listening on
	Port int

	// The multicast addresses to send our broadcasts (multicasts) out on
	MulticastAddress string

	// How long to wait before sending out another broadcast
	BroadcastDelay time.Duration

	// Maximum services to collect, -1 means there is no limit. Defaults to -1 if value is not set (ie, is zero)
	MaxServices int

	// How long to spend searching for services.
	TimeLimit time.Duration

	// By default we use UDP on IPV4. If this flag is true then we use UDP on IPV6.
	UseIPV6 bool

	// Return services found on the local host. Defaults to false.
	AllowLocal bool

	//****** Internal state ******

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

// A Service represents an address that responded to the ServiceDiscoverer broadcast.
type Service struct {
	// Address is the address of the responding service
	Address string

	// PayloadResponse is the response the service gave to our payload broadcast.
	PayloadResponse []byte
}

type broadcastFunc func(*net.UDPAddr)

// FindServices will search for services on the network by broadcasting payload to the port and
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

	return s.runCollectorAndWait()
}

func (s *ServiceDiscoverer) runCollectorAndWait() ([]Service, error) {
	// Just listen for responses
	broadcastFunc := func(udpAddr *net.UDPAddr) {
	}

	return s.runBroadcastAndCollect(broadcastFunc, nil)
}

// BroadcastService
func (s *ServiceDiscoverer) BroadcastService(ctx context.Context, payload []byte) error {
	if err := s.finishServiceDiscovererSetup(ctx, payload); err != nil {
		return err
	}

	defer s.packetConn.Close()

	_, err := s.broadcastAndCollect()
	return err
}

func (s *ServiceDiscoverer) finishServiceDiscovererSetup(ctx context.Context, payload []byte) error {
	// Carry context and payload into broadcast
	s.ctx = ctx
	s.payload = payload

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

	// Find all the host network interfaces
	networkInterfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	s.interfaces = networkInterfaces

	// Create the network connection
	addr := net.JoinHostPort(s.MulticastAddress, fmt.Sprintf("%d", s.Port))
	conn, err := net.ListenPacket(s.getUDPProtocolVersion(), addr)
	if err != nil {
		return err
	}

	// Transform connection so it is ipv4/ipv6 independent and use that
	s.packetConn = createNetPacketConn(conn, s.UseIPV6)

	// Now that we are ipv4/ipv6 independent setup the network interfaces
	setupInterfaces(s.packetConn, s.interfaces, s.MulticastAddress, s.Port)

	return nil
}

// broadcastAndCollect creates the broadcaster and address to broadcast to then delegates to
// s.runBroadcastAndCollect() to do the actual work.
func (s *ServiceDiscoverer) broadcastAndCollect() ([]Service, error) {
	// Address to broadcast on
	udpAddr := &net.UDPAddr{IP: net.ParseIP(s.MulticastAddress), Port: s.Port}
	broadcastFunc := func(udpAddr *net.UDPAddr) {
		s.broadcast(udpAddr)
	}

	return s.runBroadcastAndCollect(broadcastFunc, udpAddr)
}

// runBroadcastAndCollect will start up a background go routine to collect responses to its broadcasts. It will then
// enter a loop sending out broadcasts of payload (ServiceDiscoverer payload) on the multicast address and port.
func (s *ServiceDiscoverer) runBroadcastAndCollect(broadcastFunc broadcastFunc, udpAddr *net.UDPAddr) ([]Service, error) {
	// Create an context so we can tell the listener go routine to stop.
	ctx, cancelCollection := context.WithCancel(context.Background())

	// Start the collector in the background
	serviceCollector, err := s.startResponseCollectorInBackground(ctx)
	if err != nil {
		return nil, err
	}

	// Need our starting time so we can exit after searching for services for s.TimeLimit
	startingTime := time.Now()

	// Loop sending out broadcasts.
	// The serviceCollector will collect the responses to the broadcasts.
BroadcastLoop:
	for {
		// Send payload out on the multicast address/port
		broadcastFunc(udpAddr)

		// There are a number of conditions that determine if we should stop searching for
		// services:

		// 1. Stop if the user defined duration for finding services has been reached
		if discoveryTimeLimitReached(startingTime, s.TimeLimit) {
			break
		}

		// 2. Stop if the listener go routine has collected the user defined maximum number of
		// service responses.
		if serviceCollector.maxServicesFound {
			break
		}

		select {
		case <-s.ctx.Done():
			// 3. Stop if the ctx is done (cancel function called on context)
			break BroadcastLoop

		case <-time.After(s.BroadcastDelay): // Wait for the delay time before sending out another broadcast
		}
	}

	// At this point we've stopped broadcasting. So now we tell the service collector to shutdown
	// and then wait for it to stop
	cancelCollection()
	serviceCollector.wg.Wait()

	return serviceCollector.toServicesList(), nil
}

// startResponseCollectorInBackground starts a serviceCollector running in the background listening
// for responses to the broadcast. The context is used to tell the background listener to exit.
func (s *ServiceDiscoverer) startResponseCollectorInBackground(ctx context.Context) (*serviceCollector, error) {
	// The service collector will collect all the responses to the broadcast. It shares much of the setup
	// information with the ServiceDiscoverer so rather than duplicate that state we can just share it.
	serviceCollector := &serviceCollector{
		sd:        s,
		responses: make(map[string][]byte),
	}

	// Make sure that we can actually create the network connection that the listener go routine
	// will listen on.
	conn, err := serviceCollector.createConnection()
	if err != nil {
		return nil, err
	}

	// Start up a listener that will collect the responses from services that respond to the broadcast. Since
	// broadcastAndCollect() needs to know when the listener has exited we use a wait group to synchronize on.
	serviceCollector.wg.Add(1)
	go serviceCollector.listenForAndCollectResponses(conn, ctx)

	return serviceCollector, nil
}

// broadcast will write payload to the multicast address for all the host interfaces.
func (s *ServiceDiscoverer) broadcast(dst net.Addr) {
	for _, iface := range s.interfaces {
		if err := s.packetConn.SetMulticastInterface(&iface); err != nil {
			// log error
			continue
		}

		_ = s.packetConn.SetMulticastTTL(2)
		if _, err := s.packetConn.WriteTo(s.payload, dst); err != nil {
			// log error
		}
	}
}

// discoveryTimeLimitReached returns true if the time limit for service discovery has been reached.
func discoveryTimeLimitReached(startingTime time.Time, durationLimit time.Duration) bool {
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
