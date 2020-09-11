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

type ConnectorType string

const (
	Receiver ConnectorType = "receiver"
	Sender                 = "sender"
)

const RelayPassword = "abc123"
const RelayAppId = "relay-app-id"

type Slot struct {
	connection net.Conn
	mtype      ConnectorType
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
			case "pake":
				fmt.Println("server got pakeMsg msg")
				var pakeMsg msgs.Pake
				if err := json.Unmarshal(msg.Body, &pakeMsg); err != nil {
					fmt.Println("Unable to parse body")
					break
				}

				pw := gospake2.NewPassword(RelayPassword)
				spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS(RelayAppId))
				pakeMsgBody := spake.Start()
				pakeMsg2 := msgs.Pake{Body: pakeMsgBody}
				var sharedKey, err = spake.Finish(pakeMsg.Body)
				if err != nil {
					fmt.Println("Spake auth (finish) failed", err)
					_ = conn.Close()
					return
				}

				pakeMsg2Bytes, err := json.Marshal(pakeMsg2)
				msg2 := Message{Command: "pake", Body: pakeMsg2Bytes}
				b, _ := json.Marshal(msg2)
				if _, err := network.Write(conn, b); err != nil {
					fmt.Println("Network write failed")
					_ = conn.Close()
					return
				}

				fmt.Printf("server shared key as string = %s\n", hex.EncodeToString(sharedKey))

				_ = sharedKey

				//fmt.Printf("Hello %s\n", pakeMsg.Name)
			}
		}
	}
}

var weakKey = []byte{1, 2, 3}

func (s *Server) initializeConnection(connection net.Conn) (string, error) {

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

//func WritePake(conn net.Conn, key string) error {
//	pw := gospake2.NewPassword(key)
//	spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS("abc"))
//	pakeMsgBody := spake.Start()
//	pakeMsg := msgs.PakeMsg{Body: hex.EncodeToString(pakeMsgBody)}
//	j, err := json.Marshal(pakeMsg)
//	if err != nil {
//		return err
//	}
//	_, err = network.Write(conn, j)
//	return err
//}
//
//func ReadPake(pakeMsg msgs.PakeMsg, spake gospake2.SPAKE2) error {
//	otherSideMsg, err := hex.DecodeString(pakeMsg.Body)
//	if err != nil {
//		return err
//	}
//
//	sharedKey, err := spake.Finish(otherSideMsg)
//	_ = sharedKey
//
//	return err
//}
