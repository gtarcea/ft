package relay

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gtarcea/ft/pkg/msgs"

	"github.com/gtarcea/ft/internal/network"

	"github.com/apex/log"

	"salsa.debian.org/vasudev/gospake2"
)

const (
	Receiver = "receiver"
	Sender   = "sender"
)

const RelayPassword = "abc123"
const RelayAppId = "relay-app-id"

type Slot struct {
	connection net.Conn
	mtype      string
}

type Relay struct {
	sender     *Slot
	receiver   *Slot
	spake      *gospake2.SPAKE2
	derivedKey []byte
	opened     time.Time
	lastUsed   time.Time
	relayID    string
}

type Message struct {
	Command string `json:"command"`
	Body    []byte `json:"body"`
}

func NewRelay(senderSlot *Slot, receiverSlot *Slot) *Relay {
	return &Relay{sender: senderSlot, receiver: receiverSlot}
}

type relayList struct {
	relays map[string]*Relay
	sync.Mutex
}

type Server struct {
	relayList relayList
	port      int
	password  string
	listener  net.Listener
}

func NewRelayServer(port int, password string) *Server {
	return &Server{
		port:      port,
		password:  password,
		relayList: relayList{relays: make(map[string]*Relay)},
	}
}

func (s *Server) Start(c context.Context) error {
	// Need to track and remove any relays that are no longer active.
	go s.runInactiveRelayCleaner(c)
	return s.startRelayServer(c)
}

func (s *Server) runInactiveRelayCleaner(c context.Context) {
	for {
		select {
		case <-time.After(10 * time.Minute):
		case <-c.Done():
			log.Infof("Shutting down inactive relay cleaner...")
			return
		}

		s.removeInactiveRelays()
	}
}

func (s *Server) startRelayServer(c context.Context) error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}

	go s.runRelayServer(c)

	return nil
}

func (s *Server) runRelayServer(c context.Context) {
	tcpListener := s.listener.(*net.TCPListener)
ReadLoop:
	for {
		select {
		case <-c.Done():
			log.Infof("Shutting down relay server...")
			_ = tcpListener.Close()
			return
		default:
			_ = tcpListener.SetDeadline(time.Now().Add(2 * time.Second))
			connection, err := tcpListener.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue ReadLoop
				}
				return
			}
			go s.handleConnection(connection, c)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn, c context.Context) {
	var (
		sharedKey []byte
	)

	for {
		select {
		case <-c.Done():
			return
		default:
			var (
				msg Message
				err error
				buf []byte
			)
			if sharedKey == nil {
				buf, _, err = network.Read(conn)
				if err != nil {
					continue
				}
			} else {
				fmt.Printf("Doing network.ReadAndDecrypt\n")
				buf, _, err = network.ReadAndDecrypt(conn, sharedKey)
				if err != nil {
					fmt.Printf("ReadAndDecrypt got err: %s\n", err)
					continue
				}
			}

			if err := json.Unmarshal(buf, &msg); err != nil {
				continue
			}

			switch msg.Command {
			case "pake":
				sharedKey, err = getSharedKeyFromPakeExchange(conn, msg)
				if err != nil {
					_ = conn.Close()
					return
				}
				fmt.Printf("server shared key as string = %s\n", hex.EncodeToString(sharedKey))
			case "hello":
				err := s.handleHelloMessage(conn, msg)
				if err != nil {
					continue
				}
			}
		}
	}
}

func getSharedKeyFromPakeExchange(conn net.Conn, msg Message) ([]byte, error) {
	fmt.Println("server got pakeMsg msg")
	var pakeMsg msgs.Pake
	if err := json.Unmarshal(msg.Body, &pakeMsg); err != nil {
		fmt.Println("Unable to parse body")
		return nil, err
	}

	pw := gospake2.NewPassword(RelayPassword)
	spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS(RelayAppId))
	pakeMsgBody := spake.Start()
	pakeMsg2 := msgs.Pake{Body: pakeMsgBody}
	sharedKey, err := spake.Finish(pakeMsg.Body)
	if err != nil {
		fmt.Println("Spake auth (finish) failed", err)
		_ = conn.Close()
		return nil, err
	}

	var pakeMsg2Bytes []byte
	pakeMsg2Bytes, err = json.Marshal(pakeMsg2)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	msg2 := Message{Command: "pake", Body: pakeMsg2Bytes}
	b, _ := json.Marshal(msg2)
	if _, err := network.Write(conn, b); err != nil {
		fmt.Println("Network write failed")
		_ = conn.Close()
		return nil, err
	}

	return sharedKey, nil
}

func (s *Server) handleHelloMessage(conn net.Conn, msg Message) error {
	var hello msgs.Hello
	if err := json.Unmarshal(msg.Body, &hello); err != nil {
		return err
	}
	fmt.Printf("Got hello with relaykey: %s and connection type: %s\n", hello.RelayKey, hello.ConnectionType)
	s.relayList.Lock()
	defer s.relayList.Unlock()
	relay, ok := s.relayList.relays[hello.RelayKey]
	switch ok {
	case true:
		// Found an existing relay
		switch {
		case hello.ConnectionType == Receiver && relay.receiver != nil:
			return fmt.Errorf("already have a receiver")
		case hello.ConnectionType == Sender && relay.sender != nil:
			return fmt.Errorf("already have a sender")
		case relay.receiver != nil && relay.sender != nil:
			return fmt.Errorf("relay slots full")
		case hello.ConnectionType == Receiver:
			relay.receiver = &Slot{connection: conn, mtype: Receiver}
		case hello.ConnectionType == Sender:
			relay.sender = &Slot{connection: conn, mtype: Sender}
		default:
			// should never happen
		}

		if relay.receiver != nil && relay.sender != nil {
			// connect them together
		}

	case false:
		relay = &Relay{
			opened:   time.Now(),
			lastUsed: time.Now(),
			relayID:  hello.RelayKey,
		}

		slot := &Slot{connection: conn, mtype: hello.ConnectionType}
		if hello.ConnectionType == Sender {
			relay.sender = slot
		} else {
			relay.receiver = slot
		}

		s.relayList.relays[hello.RelayKey] = relay

	}
	return nil
}

func (s *Server) removeInactiveRelays() {
	for _, relay := range s.gatherRelaysToRemove() {
		relay.shutdown()
	}
}

func (s *Server) gatherRelaysToRemove() []*Relay {
	var relaysToRemove []*Relay

	s.relayList.Lock()
	defer s.relayList.Unlock()

	for relayKey, relay := range s.relayList.relays {
		if relay.isInactive() {
			relaysToRemove = append(relaysToRemove, relay)
			delete(s.relayList.relays, relayKey)
		}
	}

	return relaysToRemove
}

func (r *Relay) isInactive() bool {
	return time.Since(r.lastUsed) > 10*time.Minute
}

func (r *Relay) shutdown() {
	if r.receiver != nil && r.receiver.connection != nil {
		_ = r.receiver.connection.Close()
	}

	if r.sender != nil && r.sender.connection != nil {
		_ = r.sender.connection.Close()
	}
}
