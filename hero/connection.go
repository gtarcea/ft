package hero

import (
	"encoding/json"
	"fmt"
	"io"
	"net"

	"github.com/apex/log"
	"github.com/gtarcea/ft/internal/network"
)

type Message struct {
	Action string `json:"action"`
	Error  string `json:"error"`
	Body   []byte `json:"body"`
}

type connection struct {
	conn net.Conn
	ctx  *ctx
}

func newConnection(h *Hero, conn net.Conn) *connection {
	return &connection{
		ctx:  newCtx(h, conn),
		conn: conn,
	}
}

func (c *connection) handleConnection() {
	for {
		select {
		case <-c.ctx.hero.context.Done():
			_ = c.conn.Close()
			return
		default:
			msg, err := c.readMsg()
			switch {
			case err == io.EOF:
				_ = c.conn.Close()
				return
			case err != nil:
				continue
			default:
			}
			if err := c.runMsgAction(msg); err != nil {
				log.Debugf("Action returned error: %s", err)
				if _, err := c.writeError(err); err != nil {
					log.Debugf("Unable to write error to connection, got error: %s", err)
				}
			}
		}
	}
}

func (c *connection) runMsgAction(msg *Message) error {
	if action := c.getActionForMessageAction(msg.Action); action != nil {
		c.ctx.msg = msg
		if err := c.runMiddleware(); err != nil {
			return err
		}
		return action.handler(c.ctx)
	}

	return fmt.Errorf("no such action: %s", msg.Action)
}

func (c *connection) getActionForMessageAction(msgAction string) *action {
	if action, ok := c.ctx.hero.actions[msgAction]; ok && action != nil {
		return action
	}

	return nil
}

func (c *connection) runMiddleware() error {
	for i := len(c.ctx.hero.middleware); i > 0; i-- {
		if err := c.ctx.hero.middleware[i-1](c.ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *connection) readMsg() (*Message, error) {
	return ReadMsgFromConn(c.conn, c.ctx.encryptionOn, c.ctx.encryptionKey)
}

func (c *connection) writeMsg(action string, body interface{}) (int, error) {
	return WriteMsgToConn(c.conn, action, body, c.ctx.encryptionOn, c.ctx.encryptionKey)
}

func (c *connection) writeError(err error) (int, error) {
	return WriteErrorToConn(c.conn, err, c.ctx.encryptionOn, c.ctx.encryptionKey)
}

///////////////// Write ///////////////////

func WriteErrorToConn(conn net.Conn, err error, isEncrypted bool, encryptionKey []byte) (int, error) {
	m := Message{Error: fmt.Sprintf("%s", err)}
	msgBytes, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}

	if isEncrypted {
		return network.WriteEncrypted(conn, msgBytes, encryptionKey)
	}

	return network.Write(conn, msgBytes)
}

func WriteMsgToConn(conn net.Conn, action string, body interface{}, isEncrypted bool, encryptionKey []byte) (int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	m := Message{Action: action, Body: b}
	msgBytes, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}

	if isEncrypted {
		return network.WriteEncrypted(conn, msgBytes, encryptionKey)
	}

	return network.Write(conn, msgBytes)
}

///////////////// Read ///////////////////

func ReadMsgFromConn(conn net.Conn, isEncrypted bool, encryptionKey []byte) (*Message, error) {
	b, _, err := readFromConn(conn, isEncrypted, encryptionKey)
	if err != nil {
		return nil, err
	}

	var m Message
	err = json.Unmarshal(b, &m)
	return &m, err
}

func readFromConn(conn net.Conn, isEncrypted bool, encryptionKey []byte) ([]byte, int, error) {
	if isEncrypted {
		return network.ReadAndDecrypt(conn, encryptionKey)
	}

	return network.Read(conn)
}
