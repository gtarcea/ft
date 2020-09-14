package hero

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/apex/log"
)

type EncrypterFunc func(key []byte, buf []byte) ([]byte, error)
type DecrypterFunc func(key []byte, buf []byte) ([]byte, error)

type Hero struct {
	Address       string
	listener      net.Listener
	context       context.Context
	actions       map[string]*action
	EncrypterFunc EncrypterFunc
	DecrypterFunc DecrypterFunc
}

type action struct {
	name    string
	handler HandlerFunc
}

type HandlerFunc func(Context) error

type Message struct {
	Action string `json:"action"`
	Body   []byte `json:"body"`
}

func NewHero(address string) *Hero {
	return &Hero{
		Address:       address,
		actions:       make(map[string]*action),
		EncrypterFunc: defaultEncrypterFunc,
		DecrypterFunc: defaultDecrypterFunc,
	}
}

func defaultEncrypterFunc(key []byte, buf []byte) ([]byte, error) {
	return nil, fmt.Errorf("encrypter not implemented")
}

func defaultDecrypterFunc(key []byte, buf []byte) ([]byte, error) {
	return nil, fmt.Errorf("decrypter not implemented")
}

func (h *Hero) Start(ctx context.Context) error {
	var err error
	if h.listener, err = net.Listen("tcp", h.Address); err != nil {
		return err
	}
	h.acceptLoop()
	return nil
}

func (h *Hero) acceptLoop() {
	tcpListener := h.listener.(*net.TCPListener)
AcceptLoop:
	for {
		select {
		case <-h.context.Done():
			log.Infof("Shutting down...")
			_ = tcpListener.Close()
			return
		default:
			_ = tcpListener.SetDeadline(time.Now().Add(2 * time.Second))
			conn, err := tcpListener.Accept()
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue AcceptLoop
				}
				return
			}
			c := newConnection(h, conn)
			go c.handleConnection()
		}
	}
}

func (h *Hero) Shutdown() error {
	return nil
}

func (h *Hero) Action(name string, handler HandlerFunc) {
	action := &action{name: name, handler: handler}
	h.actions[name] = action
}
