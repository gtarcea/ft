package relay

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

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

	hello := msgs.Hello{Role: "receiver"}
	body, err := json.Marshal(hello)
	if err != nil {
		t.Fatalf("Couldn't marshal hello msg")
	}
	msg := Message{Command: "hello", Body: body}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Couldn't marshal message")
	}

	if _, err := network.Write(conn, b); err != nil {
		t.Fatalf("Couldn't write to server")
	}

	time.Sleep(1 * time.Second)
	cancel()
}
