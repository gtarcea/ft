package relay

import (
	"context"
	"testing"
	"time"
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
