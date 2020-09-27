package relay

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gtarcea/ft/hero"

	"salsa.debian.org/vasudev/gospake2"

	"github.com/gtarcea/ft/pkg/msgs"

	"github.com/gtarcea/ft/internal/network"
)

func TestServerStartStop(t *testing.T) {
	s := NewRelayServer(":10001", "")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := s.Start(ctx); err != nil {
			t.Fatalf("Failed to start server: %s", err)
		}
	}()
	time.Sleep(1 * time.Second)
	cancel()
	time.Sleep(3 * time.Second)
}

func TestServerPakeMsg(t *testing.T) {
	s := NewRelayServer(":10001", "")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := s.Start(ctx); err != nil {
			t.Fatalf("Failed to start server: %s", err)
		}
	}()

	time.Sleep(1 * time.Second)
	conn, err := net.DialTimeout("tcp", ":10001", 2*time.Second)
	if err != nil {
		t.Fatalf("Couldn't connect to server")
	}

	pw := gospake2.NewPassword(Password)
	spake := gospake2.SPAKE2Symmetric(pw, gospake2.NewIdentityS(AppId))
	pakeMsgBody := spake.Start()
	pake1 := msgs.Pake{Body: pakeMsgBody}
	body, err := json.Marshal(pake1)
	if err != nil {
		t.Fatalf("Couldn't marshal pake1 msg")
	}
	msg := hero.Message{Action: "pake", Body: body}
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

	var msg2 hero.Message
	if err := json.Unmarshal(buf, &msg2); err != nil {
		t.Fatalf("Unable to process message")
	}

	if msg2.Action != "pake" {
		t.Fatalf("Msg wasn't pake, got %s instead", msg2.Action)
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

	hello := msgs.Hello{RelayKey: "my-relay-key", ConnectionType: Sender}
	helloBytes, err := json.Marshal(hello)
	if err != nil {
		t.Fatalf("Unable to marshal hello message: %s", err)
	}
	msg3 := hero.Message{Action: "hello", Body: helloBytes}

	msg3Bytes, err := json.Marshal(msg3)
	if err != nil {
		t.Fatalf("Unable to marshal msg3Bytes: %s", err)
	}

	if _, err = network.WriteEncrypted(conn, msg3Bytes, sharedKey); err != nil {
		t.Fatalf("Failed writing encrypted message: %s", err)
	}

	time.Sleep(3 * time.Second)
	cancel()
}
