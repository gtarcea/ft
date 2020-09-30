package ft

import (
	"encoding/json"
	"net"
	"time"

	"github.com/gtarcea/ft/hero"
	"github.com/gtarcea/ft/internal/network"
	"github.com/gtarcea/ft/pkg/msgs"
	"github.com/pkg/errors"
	"salsa.debian.org/vasudev/gospake2"
)

type Client struct {
	RelayAddress  string
	RelayPassword string
	AppID         string

	// *** Internal State ***
	relayConn net.Conn
	relayKey  []byte
}

type ClientOpts struct {
	RelayAddress  string
	RelayPassword string
	AppID         string
}

var DefaultClientOpts ClientOpts = ClientOpts{
	RelayAddress:  ":10001",
	RelayPassword: "abc123",
	AppID:         "relay-app-id",
}

func NewClient(opts *ClientOpts) *Client {
	c := &Client{}

	if opts != nil {
		c.RelayPassword = opts.RelayPassword
		c.RelayAddress = opts.RelayAddress
		c.AppID = opts.AppID
	}

	c.setDefaults()

	return c
}

func (c *Client) setDefaults() {
	if c.RelayAddress == "" {
		c.RelayAddress = DefaultClientOpts.RelayAddress
	}

	if c.RelayPassword == "" {
		c.RelayPassword = DefaultClientOpts.RelayPassword
	}

	if c.AppID == "" {
		c.AppID = DefaultClientOpts.AppID
	}
}

func (c *Client) ConnectToRelay() error {
	var err error
	if c.relayConn, err = net.DialTimeout("tcp", c.RelayAddress, 3*time.Second); err != nil {
		return err
	}

	if err := c.exchangePake(); err != nil {
		return err
	}

	return nil
}

func (c *Client) exchangePake() error {
	pw := gospake2.NewPassword(c.RelayPassword)
	spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS(c.AppID))
	pakeMsgBody := spake.Start()
	pake1 := msgs.Pake{Body: pakeMsgBody}
	body, err := json.Marshal(pake1)
	if err != nil {
		return err
	}
	msg := hero.Message{Action: "pake", Body: body}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := network.Write(c.relayConn, b); err != nil {
		return err
	}

	buf, _, err := network.Read(c.relayConn)
	if err != nil {
		return err
	}

	var msg2 hero.Message
	if err := json.Unmarshal(buf, &msg2); err != nil {
		return err
	}

	if msg2.Action != "pake" {
		return errors.Errorf("Msg wasn't pake")
	}

	var pake2 msgs.Pake
	if err := json.Unmarshal(msg2.Body, &pake2); err != nil {
		return err
	}

	if c.relayKey, err = spake.Finish(pake2.Body); err != nil {
		return err
	}

	return nil
}

func (c *Client) WaitForReceiver() error {
	return nil
}
