package ft

import (
	"context"
	"fmt"
	"net"
)

type serviceCollector struct {
	sd               *ServiceDiscoverer
	responses        map[string][]byte
	maxServicesFound bool
}

func (s *serviceCollector) listenForAndCollectResponses(conn NetPacketConn, ctx context.Context) {
	defer conn.Close()

	buf := make([]byte, 1000)

CollectionLoop: // Loop collecting responses to broadcasts/multicasts
	for {
		n, src, err := conn.ReadFrom(buf)
		if err != nil {
			// log error
			break
		}

		srcHost, _, _ := net.SplitHostPort(src.String())
		// Make a copy of the first n bytes of buf
		bufCopy := buf[:n]
		s.responses[srcHost] = bufCopy

		if maxServicesFound(s.responses, s.sd.MaxServices) {
			break
		}

		select {
		case <-ctx.Done():
			break CollectionLoop
		}
	}
}

func maxServicesFound(servicesFound map[string][]byte, maxServices int) bool {
	if maxServices < 0 {
		return false
	}

	return len(servicesFound) >= maxServices
}

// toServicesList takes the map[string][]byte of responses and turns it into a list of
// responding addresses with their response.
func (s *serviceCollector) toServicesList() []Service {
	var servicesFound []Service

	// The ServiceCollector contains a response field which is a map where the key is the host address responding,
	// and the value is a byte array. So we iterate through the map and get the key (address) and value (payload response)
	// and copy them into the list of services that responded.
	for address, payloadResponse := range s.responses {
		service := Service{
			Address:         address,
			PayloadResponse: payloadResponse,
		}
		servicesFound = append(servicesFound, service)
	}

	return servicesFound
}

func (s *serviceCollector) createConnection(interfaces []net.Interface) (NetPacketConn, error) {
	addr := net.JoinHostPort(s.sd.MulticastAddress, fmt.Sprintf("%d", s.sd.Port))

	conn, err := net.ListenPacket(s.sd.getUDPProtocolVersion(), addr)
	if err != nil {
		// log failure
		return nil, err
	}

	packetConn := s.sd.createPacketConn(conn)
	s.sd.setupInterfaces(packetConn, interfaces)

	return packetConn, nil
}
