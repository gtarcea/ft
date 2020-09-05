package relay

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

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
	return s.runRelayServer(c)
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

func (s *Server) runRelayServer(c context.Context) error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}

	defer func() {
		_ = s.listener.Close()
	}()

	for {
		connection, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConnection(connection)
	}
}

func (s *Server) handleConnection(connection net.Conn) {
	relayKey, err := s.initializeConnection(connection)
	_ = relayKey
	_ = err
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
