package ft

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type ServiceDiscoverer struct {
	Port             int
	MulticastAddress string
	BroadcastDelay   time.Duration
	MaxServices      int
	TimeLimit        time.Duration
	UseIPV6          bool
	ctx              context.Context
	payload          []byte
}

type Service struct {
	Address         string
	PayloadResponse string
}

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

	servicesFound := s.broadcastAndCollectLoop(packetConn, networkInterfaces)

	return servicesFound, nil
}

func (s *ServiceDiscoverer) createPacketConn(conn net.PacketConn) NetPacketConn {
	if s.UseIPV6 {
		return PacketConn6{PacketConn: ipv6.NewPacketConn(conn)}
	}

	return PacketConn4{PacketConn: ipv4.NewPacketConn(conn)}
}

func (s *ServiceDiscoverer) setupInterfaces(packetConn NetPacketConn, networkInterfaces []net.Interface) {
	group := net.ParseIP(s.MulticastAddress)
	for _, ni := range networkInterfaces {
		_ = packetConn.JoinGroup(&ni, &net.UDPAddr{IP: group, Port: s.Port})
	}
}

func (s *ServiceDiscoverer) broadcastAndCollectLoop(packetConn NetPacketConn, interfaces []net.Interface) []Service {
	var servicesFound []Service
	udpAddr := &net.UDPAddr{IP: net.ParseIP(s.MulticastAddress), Port: s.Port}
	startingTime := time.Now()

	// Start up a listener that will collect the responses from services that respond to the broadcast
	go s.listenForAndCollectResponses(interfaces)

BroadcastLoop:
	for {
		broadcast(packetConn, s.payload, interfaces, udpAddr)

		if maxServicesFound(servicesFound, s.MaxServices) {
			break
		}

		if discoveryDurationReached(startingTime, s.TimeLimit) {
			break
		}

		select {
		case <-s.ctx.Done():
			break BroadcastLoop
		case <-time.After(s.BroadcastDelay):
		}
	}

	return nil
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

func maxServicesFound(servicesFound []Service, maxServices int) bool {
	if maxServices < 0 {
		return false
	}

	return len(servicesFound) >= maxServices
}

func discoveryDurationReached(startingTime time.Time, durationLimit time.Duration) bool {
	if durationLimit < 0 {
		return false
	}

	return time.Since(startingTime) > durationLimit
}

func (s *ServiceDiscoverer) listenForAndCollectResponses(interfaces []net.Interface) {
	addr := net.JoinHostPort(s.MulticastAddress, fmt.Sprintf("%d", s.Port))

	conn, err := net.ListenPacket(s.getUDPProtocolVersion(), addr)
	if err != nil {
		// log failure
		return
	}
	defer conn.Close()

	packetConn := s.createPacketConn(conn)
	s.setupInterfaces(packetConn, interfaces)

	responses := make(map[string][]byte)

	buf := make([]byte, 1000)
	for {
		n, src, err := packetConn.ReadFrom(buf)
		if err != nil {
			// log error
			return
		}

		srcHost, _, _ := net.SplitHostPort(src.String())
		// Make a copy of the first n bytes of buf
		bufCopy := buf[:n]
		responses[srcHost] = bufCopy

		//if maxServicesFound() {
		//	break
		//}
	}
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
