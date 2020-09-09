package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gtarcea/ft/internal/network"

	"github.com/apex/log"
	pake "github.com/schollz/pake/v2"
)

type ConnectorType string

const (
	Receiver ConnectorType = "receiver"
	Sender                 = "sender"
)

type Slot struct {
	connection net.Conn
	mtype      ConnectorType
}

type Relay struct {
	sender   *Slot
	receiver *Slot
	opened   time.Time
	lastUsed time.Time
}

type Message struct {
	Command string `json:"command"`
	Body    []byte `json:"body"`
}

func NewRelay(senderSlot *Slot, receiverSlot *Slot) *Relay {
	return &Relay{sender: senderSlot, receiver: receiverSlot}
}

type relayList struct {
	relays map[string]Relay
	sync.Mutex
}

type Server struct {
	relayList *relayList
	port      int
	password  string
	listener  net.Listener
}

func NewRelayServer(port int, password string) *Server {
	return &Server{
		port:      port,
		password:  password,
		relayList: &relayList{relays: make(map[string]Relay)},
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
				continue
			}
			go s.handleConnection(connection, c)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn, c context.Context) {
	for {
		select {
		case <-c.Done():
			return
		default:
			buf, _, err := network.Read(conn)
			if err != nil {
				return
			}
			var msg Message
			if err := json.Unmarshal(buf, &msg); err != nil {
				return
			}

			switch msg.Command {
			case "hello":
				fmt.Println("server got hello msg")
			}
		}
	}
}

var weakKey = []byte{1, 2, 3}

func (s *Server) initializeConnection(connection net.Conn) (string, error) {
	p, err := pake.InitCurve(weakKey, 1, "siec", 1*time.Microsecond)
	if err != nil {
		return "", err
	}

	_ = p
	return "", nil
}

func (s *Server) removeInactiveRelays() {
	for _, relay := range s.gatherRelaysToRemove() {
		relay.shutdown()
	}
}

func (s *Server) gatherRelaysToRemove() []Relay {
	var relaysToRemove []Relay

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

func (r Relay) isInactive() bool {
	return time.Since(r.lastUsed) > 10*time.Minute
}

func (r Relay) shutdown() {
	if r.receiver != nil && r.receiver.connection != nil {
		_ = r.receiver.connection.Close()
	}

	if r.sender != nil && r.sender.connection != nil {
		_ = r.sender.connection.Close()
	}
}
