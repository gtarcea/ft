package relay

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"salsa.debian.org/vasudev/gospake2"

	"github.com/gtarcea/ft/pkg/msgs"

	"github.com/gtarcea/ft/internal/network"
)

func TestServerStartStop(t *testing.T) {
	s := NewRelayServer(10001, "")
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %s", err)
	}
	time.Sleep(1 * time.Second)
	cancel()
	time.Sleep(3 * time.Second)
}

func TestServerHelloMsg(t *testing.T) {
	s := NewRelayServer(10001, "")
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %s", err)
	}

	conn, err := net.DialTimeout("tcp", ":10001", 2*time.Second)
	if err != nil {
		t.Fatalf("Couldn't connect to server")
	}

	pw := gospake2.NewPassword(RelayPassword)
	spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS(RelayAppId))
	pakeMsgBody := spake.Start()
	pake1 := msgs.Pake{Body: pakeMsgBody}
	body, err := json.Marshal(pake1)
	if err != nil {
		t.Fatalf("Couldn't marshal pake1 msg")
	}
	msg := Message{Command: "pake", Body: body}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Couldn't marshal message")
	}

	if _, err := network.Write(conn, b); err != nil {
		t.Fatalf("Couldn't write to server")
	}

	buf, _, err := network.Read(conn)
	if err != nil {
		t.Fatalf("Unable to read response")
	}

	var msg2 Message
	if err := json.Unmarshal(buf, &msg2); err != nil {
		t.Fatalf("Unable to process message")
	}

	if msg2.Command != "pake" {
		t.Fatalf("Msg wasn't welcome, got %s instead", msg2.Command)
	}

	var pake2 msgs.Pake
	if err := json.Unmarshal(msg2.Body, &pake2); err != nil {
		t.Fatalf("Unable to unmarshal body %s", err)
	}

	sharedKey, err := spake.Finish(pake2.Body)
	if err != nil {
		t.Fatalf("Spake auth (finish) failed %s", err)
	}

	fmt.Printf("Client shared key as string = %s\n", hex.EncodeToString(sharedKey))

	time.Sleep(3 * time.Second)
	cancel()
}

func TestServerPakeMsg(t *testing.T) {
	s := NewRelayServer(10001, "")
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %s", err)
	}

	conn, err := net.DialTimeout("tcp", ":10001", 2*time.Second)
	if err != nil {
		t.Fatalf("Couldn't connect to server")
	}

	_ = conn
	time.Sleep(2 * time.Second)
	cancel()
}
