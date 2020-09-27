package hero

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
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
	WriteMsg(action string, body interface{}) error
	ReadMsg(i interface{}) error
	Action() string
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
	_, err := WriteMsgToConn(c.conn, action, value, c.encryptionOn, c.encryptionKey)
	return err
}

func (c *ctx) WriteMsg(action string, value interface{}) error {
	return c.JSON(action, value)
}

func (c *ctx) ReadMsg(i interface{}) error {
	msg, err := ReadMsgFromConn(c.conn, c.encryptionOn, c.encryptionKey)
	if err != nil {
		return err
	}

	return json.Unmarshal(msg.Body, i)
}

func (c *ctx) Action() string {
	return c.msg.Action
}
