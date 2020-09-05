package internal

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/apex/log"
)

type ConnectorType string

const (
	Receiver ConnectorType = "receiver"
	Sender                 = "sender"
)

type RelaySlot struct {
	connection net.Conn
	mtype      ConnectorType
}

type Relay struct {
	SenderSlot   *RelaySlot
	ReceiverSlot *RelaySlot
	opened       time.Time
	lastUsed     time.Time
}

func NewRelay(senderSlot *RelaySlot, receiverSlot *RelaySlot) *Relay {
	return &Relay{SenderSlot: senderSlot, ReceiverSlot: receiverSlot}
}

type Mailbox struct {
	relays map[string]Relay
	sync.Mutex
}

type RelayServer struct {
	mailbox  *Mailbox
	port     int
	password string
}

func NewRelayServer(port int, password string) *RelayServer {
	return &RelayServer{
		port:     port,
		password: password,
		mailbox:  &Mailbox{relays: make(map[string]Relay)},
	}
}

func (s *RelayServer) Start(c context.Context) error {
	// Need to track and remove any relays that are no longer active.
	go s.runInactiveRelayCleaner(c)
	return nil
}

func (s *RelayServer) runInactiveRelayCleaner(c context.Context) {
	for {
		select {
		case <-time.After(10 * time.Minute):
		case <-c.Done():
			log.Infof("Shutting down file loading...")
			return
		}

		s.removeInactiveRelays()
	}
}

func (s *RelayServer) removeInactiveRelays() {
	relaysToPotentiallyRemove := s.gatherCandidatesForRemoval()
	s.cleanupAndRemoveRelays(relaysToPotentiallyRemove)
}

func (s *RelayServer) gatherCandidatesForRemoval() []string {
	var relaysToPotentiallyRemove []string
	s.mailbox.Lock()
	for relayKey := range s.mailbox.relays {
		if relayIsCandidateForRemoval(s.mailbox.relays[relayKey]) {
			relaysToPotentiallyRemove = append(relaysToPotentiallyRemove, relayKey)
		}
	}
	s.mailbox.Unlock()

	return relaysToPotentiallyRemove
}

func (s *RelayServer) cleanupAndRemoveRelays(relayKeys []string) {
	for _, relayKey := range relayKeys {
		s.cleanupRelay(relayKey)
	}
}

func relayIsCandidateForRemoval(relay Relay) bool {
	return time.Since(relay.lastUsed) > 3*time.Minute
}

func (s *RelayServer) cleanupRelay(relayKey string) {
	s.mailbox.Lock()
	defer s.mailbox.Unlock()

	relay, ok := s.mailbox.relays[relayKey]

	if !ok {
		// relay was already removed
		return
	}

	if relayIsCandidateForRemoval(relay) {
		if relay.ReceiverSlot != nil && relay.ReceiverSlot.connection != nil {
			_ = relay.ReceiverSlot.connection.Close()
		}

		if relay.SenderSlot != nil && relay.SenderSlot.connection != nil {
			_ = relay.SenderSlot.connection.Close()
		}

		delete(s.mailbox.relays, relayKey)
	}
}
