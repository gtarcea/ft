package network

import (
	"bytes"
	"encoding/binary"
	"net"
	"time"
)

type connection struct {
	conn net.Conn
}

func (c *connection) Write(b []byte) (int, error) {
	header := new(bytes.Buffer)
	// write header which is the length of the buffer we are sending
	err := binary.Write(header, binary.LittleEndian, uint32(len(b)))
	if err != nil {
		// do something like log it
	}

	buffer := append(header.Bytes(), b...)
	return c.conn.Write(buffer)
}

func (c *connection) Send(b []byte) error {
	if _, err := c.Write(b); err != nil {
		return err
	}

	return nil
}

func (c *connection) Read() (buf []byte, numBytes int, err error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(3 * time.Hour)); err != nil {
		// log it
	}

	// read header header will test us how long the rest of the content is
	var header []byte
	numBytes = 4
	for {
		tmp := make([]byte, numBytes-len(header))
		n, errRead := c.conn.Read(tmp)
		if errRead != nil {
			return nil, numBytes, errRead
		}
		header = append(header, tmp[:n]...)
		if numBytes == len(header) {
			break
		}
	}

	return nil, 4, nil
}
