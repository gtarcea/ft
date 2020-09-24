package ft

import (
	"context"
	"fmt"
	"net"
	"sync"
)

// A serviceCollector collects the broadcast services on the address/port being listened on.
type serviceCollector struct {
	// Local copy of the ServiceDiscoverer we reuse for some shared settings and state
	sd *ServiceDiscoverer

	// A map of responses where the key is the response IP and the value is the response
	responses map[string][]byte

	// collectorFinished is used to signal that the collector exited. Currently it can exit
	// early because ctx.Done() returned, because it collected the maximum number of services
	// or because the call OnServiceFoundFunc returned true.
	collectorFinished bool

	// Used by the serviceCollector to let other go routines wait for the collector to finish.
	// This allows other routes to call wg.Wait() and when that returns they know the collector
	// has exited and that its safe to access the responses field.
	wg sync.WaitGroup
}

// listForAndCollectResponses listens for responses on the local interfaces. It checks if the response
// is from a valid host and if it is tracks that response. This method will set s.wg.Done() when it is
// finished to signal that it is safe to read the map of reponses.
func (s *serviceCollector) listenForAndCollectResponses(conn NetPacketConn, ctx context.Context) {
	defer conn.Close()

	defer s.wg.Done() // Signal we are done collecting

	// Load up the list of local addresses for the host. This is used to track whether a response
	// came from the local host and works in conjunction with the ServiceDiscoverer AllowLocal flag
	// to determine if we should collect that response.
	localAddressTracker := newLocalAddressTracker(s.sd.interfaces)

	// Buffer that will contain the response from the multicast connection.
	buf := make([]byte, 1000)

	// Loop collecting responses to broadcasts/multicasts
CollectionLoop:
	for {
		// Check if we are done. This is the first check in the loop because other checks in the loop
		// have continue in them. If this check were at the bottom then it would be bypassed by the
		// continue restarting the loop at the top. Potentially that would mean we never check to see
		// if the context was cancelled and we should exit early.

		select {
		case <-ctx.Done():
			break CollectionLoop
		default:
		}

		// TODO: Add read timeout and check if error is read timeout
		n, src, err := conn.ReadFrom(buf)
		if err != nil {
			// log error
			break
		}

		srcHost, _, err := net.SplitHostPort(src.String())
		if err != nil {
			continue
		}

		if localAddressTracker.isLocalAddress(srcHost) && !s.sd.AllowLocal {
			// Address belongs to host we are running on and we aren't collecting any local services
			continue
		}

		bufCopy := buf[:n] // Make a copy of the first n bytes of buf

		// Add responding host to responses
		s.responses[srcHost] = bufCopy

		if s.sd.OnServiceFoundFunc(&Service{Address: srcHost, PayloadResponse: bufCopy}) {
			s.collectorFinished = true
			break
		}

		if maxServicesFound(s.responses, s.sd.MaxServices) {
			s.collectorFinished = true
			break
		}
	}
}

// maxServicesFound returns true if the user specified maximum number of services was found. It handle the
// case where the user specified unlimited service collection.
func maxServicesFound(servicesFound map[string][]byte, maxServices int) bool {
	if maxServices < 0 {
		// Unlimited service collection
		return false
	}

	return len(servicesFound) >= maxServices
}

// toServicesList takes the map[string][]byte of responses and turns it into a list of
// responding addresses with their response.
func (s *serviceCollector) toServicesList() []Service {
	var servicesFound []Service

	// The ServiceCollector contains a response field which is a map where the key is the host address responding,
	// and the value is a byte array (the response sent). So we iterate through the map and get the key (address)
	// and value (payload response) and copy them into the list of services that responded.
	for address, payloadResponse := range s.responses {
		service := Service{
			Address:         address,
			PayloadResponse: payloadResponse,
		}
		servicesFound = append(servicesFound, service)
	}

	return servicesFound
}

// createConnection creates the new connection, maps it into a NetPacketConn and sets up the interfaces
// to listen on.
func (s *serviceCollector) createConnection() (NetPacketConn, error) {
	addr := net.JoinHostPort(s.sd.MulticastAddress, fmt.Sprintf("%d", s.sd.Port))

	conn, err := net.ListenPacket(getUDPProtocolVersion(s.sd.UseIPV6), addr)
	if err != nil {
		// log failure
		return nil, err
	}

	packetConn := createNetPacketConn(conn, s.sd.UseIPV6)
	setupInterfaces(packetConn, s.sd.interfaces, s.sd.MulticastAddress, s.sd.Port)

	return packetConn, nil
}

// localAddressTracker contains a map of all addresses that are local to the system. This is used to
// identify responses that came from the local host and to filter them out if ServiceDiscoverer.LocalAllowed
// is set to false.
type localAddressTracker struct {
	localAddressMap map[string]bool
}

// newLocalAddressTracker creates a new localAddressTracker and loads it with all the local
// host addresses. These include common ones such as 'localhost', '127.0.0.1', and (for ipv6)
// '::1'. It also goes through the list of interfaces, finds their local addresses and adds
// them to the map.
func newLocalAddressTracker(interfaces []net.Interface) *localAddressTracker {
	var localAddressMap = make(map[string]bool)

	// Setup local address IPs for IPv4 and IPv6 as well as localhost name
	localAddressMap["localhost"] = true
	localAddressMap["127.0.0.1"] = true
	localAddressMap["::1"] = true

	// loop through all the local network interfaces extracting addresses and adding
	// them into the localAddressMap
	for _, networkInterface := range interfaces {
		// Get addresses for our local host interface
		addrs, err := networkInterface.Addrs()
		if err != nil {
			continue
		}

		// Add the addresses for the interface
		for _, addr := range addrs {
			ipAddress, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			localAddressMap[ipAddress.String()] = true
		}
	}

	return &localAddressTracker{localAddressMap: localAddressMap}
}

// isLocalAddress is a convenience function for checking if a give address is
// in the local address map.
func (t *localAddressTracker) isLocalAddress(addr string) bool {
	_, ok := t.localAddressMap[addr]
	return ok
}
