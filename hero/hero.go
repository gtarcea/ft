package hero

import (
	"context"
	"net"
)

type Hero struct {
	Address       string
	listener      net.Listener
	ctx           context.Context
	actions       map[string]*action
	encryptionKey []byte
}

type action struct {
	name      string
	encrypted bool
	handler   HandlerFunc
}

type HandlerFunc func(Context) error

type Context struct{}

type Message struct {
	Action string `json:"action"`
	Body   []byte `json:"body"`
}

func NewHero(address string) *Hero {
	return &Hero{Address: address, actions: make(map[string]*action)}
}

func (h *Hero) Start(ctx context.Context) error {
	h.ctx = ctx
	return nil
}

func (h *Hero) Shutdown() error {
	return nil
}

func (h *Hero) Action(name string, handler HandlerFunc) {
	action := &action{name: name, encrypted: false, handler: handler}
	h.actions[name] = action
}

func (h *Hero) EncryptedAction(name string, handler HandlerFunc) {
	action := &action{name: name, encrypted: true, handler: handler}
	h.actions[name] = action
}
