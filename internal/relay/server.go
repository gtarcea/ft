package relay

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gtarcea/ft/pkg/ft"

	"github.com/gtarcea/ft/hero"
	"github.com/gtarcea/ft/pkg/msgs"
	"salsa.debian.org/vasudev/gospake2"
)

const (
	Receiver = "receiver"
	Sender   = "sender"
)

const Password = "abc123"
const AppId = "relay-app-id"

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

type relayList struct {
	relays map[string]*Relay
	sync.Mutex
}

type Server struct {
	relayList relayList
	address   string
	password  string
	states    *ft.State
}

func NewServer(address string, password string) *Server {
	server := &Server{
		address:   address,
		password:  password,
		relayList: relayList{relays: make(map[string]*Relay)},
		states:    ft.NewState(),
	}

	server.states.AddState("start", "pake")
	server.states.AddState("pake", "hello")
	server.states.AddState("hello", "external_ips", "go")
	server.states.AddState("external_ips", "go")
	server.states.SetStartState("start")

	return server
}

func (s *Server) Start(c context.Context) error {
	h := hero.NewHero(s.address)
	h.AddMiddleware(s.validStateMiddleware)
	h.Action("pake", s.authenticateHandler)
	h.Action("hello", s.helloHandler)
	h.Action("ready", s.readyHandler)
	return h.Start(c)
}

func (s *Server) validStateMiddleware(c hero.Context) error {
	return s.states.ValidateAndAdvanceToNextState(c.Action())
}

func (s *Server) authenticateHandler(c hero.Context) error {
	var pakeMsg msgs.Pake
	if err := c.Bind(&pakeMsg); err != nil {
		fmt.Println("Unable to parse body")
		return err
	}

	pw := gospake2.NewPassword(Password)
	spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS(AppId))
	pakeMsgBody := spake.Start()
	sharedKey, err := spake.Finish(pakeMsg.Body)

	if err != nil {
		fmt.Println("Spake auth (finish) failed", err)
		return err
	}
	pakeMsg2 := msgs.Pake{Body: pakeMsgBody}
	if err := c.JSON("pake", pakeMsg2); err != nil {
		fmt.Println("failed writing return pake msg:", err)
		return err
	}

	c.SetEncryptionKey(sharedKey)
	return c.TurnEncryptionOn()
}

func (s *Server) helloHandler(c hero.Context) error {
	var hello msgs.Hello
	if err := c.Bind(&hello); err != nil {
		fmt.Println("Failed binding hello message:", err)
	}

	fmt.Printf("Got hello with relaykey: %s and connection type: %s\n", hello.RelayKey, hello.ConnectionType)

	s.relayList.Lock()
	defer s.relayList.Unlock()
	relay, foundRelay := s.relayList.relays[hello.RelayKey]

	if foundRelay {
		// Found an existing relay
		switch {
		case hello.ConnectionType == Receiver && relay.receiver != nil:
			return fmt.Errorf("already have a receiver")
		case hello.ConnectionType == Sender && relay.sender != nil:
			return fmt.Errorf("already have a sender")
		case relay.receiver != nil && relay.sender != nil:
			return fmt.Errorf("relay slots full")
		case hello.ConnectionType == Receiver:
			relay.receiver = &Slot{connection: c.Conn(), mtype: Receiver}
		case hello.ConnectionType == Sender:
			relay.sender = &Slot{connection: c.Conn(), mtype: Sender}
		default:
			// should never happen
		}

		return nil
	}

	// No relay found so create one

	relay = &Relay{
		opened:   time.Now(),
		lastUsed: time.Now(),
		relayID:  hello.RelayKey,
	}

	slot := &Slot{connection: c.Conn(), mtype: hello.ConnectionType}
	if hello.ConnectionType == Sender {
		relay.sender = slot
	} else {
		relay.receiver = slot
	}

	s.relayList.relays[hello.RelayKey] = relay

	return nil
}

func (s *Server) readyHandler(c hero.Context) error {
	_ = c
	return nil
}
