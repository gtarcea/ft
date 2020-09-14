package hero

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/gtarcea/ft/hero/internal/network"
)

type Context interface {
	SetEncryptionKey(key []byte)
	GetEncryptionKey() []byte
	TurnEncryptionOn() error
	TurnEncryptionOff() error
	RemoteAddr() net.Addr
	Get(key string) interface{}
	Set(key string, value interface{})
	Bind(i interface{}) error
	Hero() *Hero
	Conn() net.Conn
	JSON(string, interface{}) error
}

type ctx struct {
	hero          *Hero
	store         *sync.Map
	conn          net.Conn
	msg           *Message
	encryptionKey []byte
	encryptionOn  bool
}

func newCtx(hero *Hero, conn net.Conn) *ctx {
	return &ctx{hero: hero, conn: conn}
}

func (c *ctx) SetEncryptionKey(key []byte) {
	c.encryptionKey = key
}

func (c *ctx) GetEncryptionKey() []byte {
	return c.encryptionKey
}

func (c *ctx) TurnEncryptionOn() error {
	if c.encryptionKey == nil {
		return fmt.Errorf("no encryption key")
	}

	c.encryptionOn = true
	return nil
}

func (c *ctx) TurnEncryptionOff() error {
	if c.encryptionKey == nil {
		return fmt.Errorf("no encryption key")
	}

	c.encryptionOn = false
	return nil
}

func (c *ctx) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *ctx) Get(key string) interface{} {
	val, _ := c.store.Load(key)
	return val
}

func (c *ctx) Set(key string, value interface{}) {
	c.store.Store(key, value)
}

func (c *ctx) Bind(i interface{}) error {
	return json.Unmarshal(c.msg.Body, i)
}

func (c *ctx) Hero() *Hero {
	return c.hero
}

func (c *ctx) Conn() net.Conn {
	return c.conn
}

func (c *ctx) JSON(action string, value interface{}) error {
	_, err := c.writeMsg(action, value)
	return err
}

func (c *ctx) writeMsg(action string, body interface{}) (int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	m := Message{Action: action, Body: b}
	msgBytes, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}

	return c.write(msgBytes)
}

func (c *ctx) write(b []byte) (int, error) {
	if c.encryptionOn {
		return network.WriteEncrypted(c.conn, b, c.encryptionKey)
	}

	return network.Write(c.conn, b)
}
