package relayer

import (
	"net"
	"time"
)

type Client struct {
	relayId    string
	address    string
	connection net.Conn
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
