package ft

import (
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// createNetPacketConn takes a net.PacketConn and maps it into a structure that conforms to
// NetPacketConn interface. This allows us to use a single interface to handle both ipv6 and
// ipv4 connections. The implementations for these connections have slightly different calls,
// which these structures map over to create a unified interface.
func createNetPacketConn(conn net.PacketConn, useIPV6 bool) NetPacketConn {
	if useIPV6 {
		return PacketConn6{PacketConn: ipv6.NewPacketConn(conn)}
	} else {
		return PacketConn4{PacketConn: ipv4.NewPacketConn(conn)}
	}
}

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
