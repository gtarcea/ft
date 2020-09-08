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

	// Now write the buffer
	buffer := append(header.Bytes(), b...)
	return c.conn.Write(buffer)
}

func (c *connection) Send(b []byte) error {
	if _, err := c.Write(b); err != nil {
		return err
	}

	return nil
}

func (c *connection) Read() ([]byte, int, error) {
	bufSize, err := c.readHeaderBufSize()
	_ = err
	return nil, bufSize, nil
}

func (c *connection) readHeaderBufSize() (int, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(3 * time.Hour)); err != nil {
		// log it
	}

	var header []byte
	numBytes := 4
	for {
		tmp := make([]byte, numBytes-len(header))
		n, err := c.conn.Read(tmp)
		if err != nil {
			return n, err
		}
		header = append(header, tmp[:n]...)
		if numBytes == len(header) {
			break
		}
	}

	return convertHeaderBytesToInt(header)
}

func convertHeaderBytesToInt(header []byte) (int, error) {
	var bufSize uint32
	if err := binary.Read(bytes.NewReader(header), binary.LittleEndian, &bufSize); err != nil {
		return 0, err
	}

	return int(bufSize), nil
}
