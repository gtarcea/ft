package ft

import (
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// The NetPacketConn interface is used to turn ipv4 and ipv6 PacketConn into a single interface since the
// two implementations are slightly different. The methods below are all methods that are needed on
// a net.PacketConn that we map to our interface.
type NetPacketConn interface {
	// Close ignores the error on closing the connection (at the point we are closing we don't care if there
	// is an issue with the network close).
	Close()
	JoinGroup(ifi *net.Interface, group net.Addr) error
	SetMulticastInterface(ini *net.Interface) error

	// SetMulticastTTL is the first difference between ipv4 and ipv6. The ipv4 implementation
	// uses SetMulticastTTL() while ipv6 uses SetMulticastHopLimit()
	SetMulticastTTL(int) error

	// Both ipv4 and ipv6 implement ReadFrom, but they each return their own implementation of
	// ControlMessage (ipv4.ControlMessage or ipv6.ControlMessage). Here we map to a function that
	// doesn't return this (for us) unneeded return.
	ReadFrom(buf []byte) (int, net.Addr, error)

	// Both ipv4 and ipv6 implement WriteTo, but they each take their own package specific ControlMessage.
	// Again, since these aren't needed and it breaks having a common interface we create our own WriteTo
	// method to conform to a common interface.
	WriteTo(buf []byte, dst net.Addr) (int, error)
}

// ipv4 implementation of NetPacketConn
type NetPacketConn4 struct {
	*ipv4.PacketConn
}

// Close wraps the ipv4 Close and ignores the error
func (p NetPacketConn4) Close() {
	_ = p.PacketConn.Close()
}

// ReadFrom wraps the ipv4 ReadFrom and ignores the ControlMessage
func (p NetPacketConn4) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, _, addr, err := p.PacketConn.ReadFrom(buf)
	return n, addr, err
}

// WriteTo wraps the ipv4 WriteTo call and passes nil for the ControlMessage
func (p NetPacketConn4) WriteTo(buf []byte, dst net.Addr) (int, error) {
	return p.PacketConn.WriteTo(buf, nil, dst)
}

// ipv6 implementation of NetPacketConn
type NetPacketConn6 struct {
	*ipv6.PacketConn
}

// Close wraps the ipv4 Close and ignores the error
func (p NetPacketConn6) Close() {
	_ = p.PacketConn.Close()
}

// ReadFrom wraps the ipv6 ReadFrom and ignores the ControlMessage
func (p NetPacketConn6) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, _, addr, err := p.PacketConn.ReadFrom(buf)
	return n, addr, err
}

// WriteTo wraps the ipv6 WriteTo call and passes nil for the ControlMessage
func (p NetPacketConn6) WriteTo(buf []byte, dst net.Addr) (int, error) {
	return p.PacketConn.WriteTo(buf, nil, dst)
}

// SetMulticastTTL calls the ipv6 SetMulticastHopLimit instead
func (p NetPacketConn6) SetMulticastTTL(i int) error {
	return p.SetMulticastHopLimit(i)
}

// createNetPacketConn takes a net.PacketConn and maps it into a structure that conforms to
// NetPacketConn interface. This allows us to use a single interface to handle both ipv6 and
// ipv4 connections. The implementations for these connections have slightly different calls,
// which these structures map over to create a unified interface.
func createNetPacketConn(conn net.PacketConn, useIPV6 bool) NetPacketConn {
	if useIPV6 {
		return NetPacketConn6{PacketConn: ipv6.NewPacketConn(conn)}
	} else {
		return NetPacketConn4{PacketConn: ipv4.NewPacketConn(conn)}
	}
}
