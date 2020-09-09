package network

import (
	"bytes"
	"encoding/binary"
	"net"
	"time"
)

func Write(conn net.Conn, b []byte) (int, error) {
	header := new(bytes.Buffer)
	// write header which is the length of the buffer we are sending
	err := binary.Write(header, binary.LittleEndian, uint32(len(b)))
	if err != nil {
		// do something like log it
	}

	// Append header (buffer size) and buffer together and write
	buffer := append(header.Bytes(), b...)
	return conn.Write(buffer)
}

func Read(conn net.Conn) ([]byte, int, error) {
	bufSize, err := readHeaderBufSize(conn)
	if err != nil {
		return nil, 0, err
	}

	buf := make([]byte, 0)
	// Loop until we read bufSize or get an error
	for {
		// Allocate up to what we have already read, we start at a tmpBuf size equal
		// to bufSize, and then shorten if we don't read that amount
		tmpBuf := make([]byte, bufSize-len(buf))
		n, err := conn.Read(tmpBuf)
		switch {
		case err != nil:
			return nil, 0, err

		default:
			buf = append(buf, tmpBuf[:n]...)
			if len(buf) == bufSize {
				// we've read the amount expected
				return buf, bufSize, nil
			}
		}
	}
}

func readHeaderBufSize(conn net.Conn) (int, error) {
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Hour)); err != nil {
		// log it
	}

	var header []byte
	numBytes := 4
	for {
		tmp := make([]byte, numBytes-len(header))
		n, err := conn.Read(tmp)
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
