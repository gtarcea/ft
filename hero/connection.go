package hero

import (
	"fmt"
	"net"

	"github.com/apex/log"
)

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
			return
		default:
			msg, err := c.readMsg()
			if err != nil {
				continue
			}
			if err := c.runMsgAction(msg); err != nil {
				log.Debugf("Action returned error: %s", err)
			}
		}
	}
}

func (c *connection) runMsgAction(msg *Message) error {
	if action, ok := c.ctx.hero.actions[msg.Action]; ok && action != nil {
		c.ctx.msg = msg
		return action.handler(c.ctx)
	}

	return fmt.Errorf("no such action: %s", msg.Action)
}

func (c *connection) readMsg() (*Message, error) {
	return ReadMsgFromConn(c.conn, c.ctx.encryptionOn, c.ctx.encryptionKey)
}

func (c *connection) writeMsg(action string, body interface{}) (int, error) {
	return WriteMsgToConn(c.conn, action, body, c.ctx.encryptionOn, c.ctx.encryptionKey)
}
