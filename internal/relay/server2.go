package relay

import (
	"context"
	"fmt"
	"time"

	hero2 "github.com/gtarcea/ft/hero"
	"github.com/gtarcea/ft/pkg/msgs"
	"salsa.debian.org/vasudev/gospake2"
)

type Server2 struct {
	relayList relayList
	address   string
	password  string
}

func NewRelayServer2(address string, password string) *Server2 {
	return &Server2{
		address:   address,
		password:  password,
		relayList: relayList{relays: make(map[string]*Relay)},
	}
}

func (s *Server2) Start(c context.Context) error {
	hero := hero2.NewHero(s.address)
	hero.Action("pake", s.authenticateHandler)
	hero.Action("hello", s.helloHandler)
	return hero.Start(c)
}

func (s *Server2) authenticateHandler(c hero2.Context) error {
	var pakeMsg msgs.Pake
	if err := c.Bind(&pakeMsg); err != nil {
		fmt.Println("Unable to parse body")
		return err
	}

	pw := gospake2.NewPassword(RelayPassword)
	spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS(RelayAppId))
	pakeMsgBody := spake.Start()
	pakeMsg2 := msgs.Pake{Body: pakeMsgBody}
	sharedKey, err := spake.Finish(pakeMsg.Body)

	if err != nil {
		fmt.Println("Spake auth (finish) failed", err)
		return err
	}

	if err := c.JSON("pake", pakeMsg2); err != nil {
		fmt.Println("failed writing return pake msg:", err)
		return err
	}

	c.SetEncryptionKey(sharedKey)
	_ = c.TurnEncryptionOn()

	return nil
}

func (s *Server2) helloHandler(c hero2.Context) error {
	var hello msgs.Hello
	if err := c.Bind(&hello); err != nil {
		fmt.Println("Failed binding hello message:", err)
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
			relay.receiver = &Slot{connection: c.Conn(), mtype: Receiver}
		case hello.ConnectionType == Sender:
			relay.sender = &Slot{connection: c.Conn(), mtype: Sender}
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

		slot := &Slot{connection: c.Conn(), mtype: hello.ConnectionType}
		if hello.ConnectionType == Sender {
			relay.sender = slot
		} else {
			relay.receiver = slot
		}

		s.relayList.relays[hello.RelayKey] = relay

	}
	return nil
}
