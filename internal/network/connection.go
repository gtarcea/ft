package network

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
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

func WriteEncrypted(conn net.Conn, b []byte, key []byte) (int, error) {
	var (
		err         error
		cipherBlock cipher.Block
		gcm         cipher.AEAD
	)
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return 0, err
	}

	if cipherBlock, err = aes.NewCipher(key); err != nil {
		return 0, err
	}

	if gcm, err = cipher.NewGCM(cipherBlock); err != nil {
		return 0, err
	}

	encryptedBytes := gcm.Seal(nil, nonce, b, nil)
	encryptedBytes = append(nonce, encryptedBytes...)
	return Write(conn, encryptedBytes)
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

func ReadAndDecrypt(conn net.Conn, key []byte) ([]byte, int, error) {
	var (
		cipherBlock      cipher.Block
		gcm              cipher.AEAD
		err              error
		encryptedBytes   []byte
		unencryptedBytes []byte
		n                int
	)

	if encryptedBytes, n, err = Read(conn); err != nil {
		fmt.Println("Read failed")
		return nil, 0, err
	}

	if cipherBlock, err = aes.NewCipher(key); err != nil {
		fmt.Println("NewCipher failed")
		return nil, 0, err
	}

	if gcm, err = cipher.NewGCM(cipherBlock); err != nil {
		fmt.Println("NewGCM failed")
		return nil, 0, err
	}

	unencryptedBytes, err = gcm.Open(nil, encryptedBytes[:12], encryptedBytes[12:], nil)
	if err != nil {
		fmt.Println("gcm.Open failed")
	}

	return unencryptedBytes, n - 12, err
}
