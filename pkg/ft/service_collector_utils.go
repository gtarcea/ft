package ft

import (
	"net"
)

// setupInterfaces iterates through the list of interfaces and sets the UDP address (group) to use for that interface
func setupInterfaces(packetConn NetPacketConn, interfaces []net.Interface, multicastAddress string, port int) {
	group := net.ParseIP(multicastAddress)
	for _, ni := range interfaces {
		_ = packetConn.JoinGroup(&ni, &net.UDPAddr{IP: group, Port: port})
	}
}

// getUDPProtocolVersion returns the correct protocol version string depending on whether we are
// using IPv4 or IPv6.
func getUDPProtocolVersion(useIPV6 bool) string {
	if useIPV6 {
		return "udp6"
	}

	return "udp4"
}
