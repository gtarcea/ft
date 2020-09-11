package relayer

import (
	"net"
	"time"

	"salsa.debian.org/vasudev/gospake2"
)

type Client struct {
	relayId    string
	address    string
	connection net.Conn
	spake      *gospake2.SPAKE2
	sharedKey  string
	password   string
	appID      string
}

func NewClient(relayId, address string) (*Client, error) {
	connection, err := net.DialTimeout("tcp", address, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return &Client{relayId: relayId, address: address, connection: connection}, nil
}

func NewClientForConnection(connection net.Conn, relayId, address string) *Client {
	return &Client{relayId: relayId, address: address, connection: connection}
}
