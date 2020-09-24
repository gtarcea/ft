package ft

import (
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type NetPacketConn interface {
	Close() error
	JoinGroup(ifi *net.Interface, group net.Addr) error
	SetMulticastInterface(ini *net.Interface) error
	SetMulticastTTL(int) error
	ReadFrom(buf []byte) (int, net.Addr, error)
	WriteTo(buf []byte, dst net.Addr) (int, error)
}

type PacketConn4 struct {
	*ipv4.PacketConn
}

// ReadFrom wraps the ipv4 ReadFrom without a control message
func (pc4 PacketConn4) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, _, addr, err := pc4.PacketConn.ReadFrom(buf)
	return n, addr, err
}

// WriteTo wraps the ipv4 WriteTo without a control message
func (pc4 PacketConn4) WriteTo(buf []byte, dst net.Addr) (int, error) {
	return pc4.PacketConn.WriteTo(buf, nil, dst)
}

type PacketConn6 struct {
	*ipv6.PacketConn
}

// ReadFrom wraps the ipv6 ReadFrom without a control message
func (pc6 PacketConn6) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, _, addr, err := pc6.PacketConn.ReadFrom(buf)
	return n, addr, err
}

// WriteTo wraps the ipv6 WriteTo without a control message
func (pc6 PacketConn6) WriteTo(buf []byte, dst net.Addr) (int, error) {
	return pc6.PacketConn.WriteTo(buf, nil, dst)
}

// SetMulticastTTL wraps the hop limit of ipv6
func (pc6 PacketConn6) SetMulticastTTL(i int) error {
	return pc6.SetMulticastHopLimit(i)
}

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
