package ft

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServiceDiscovererFindsLocalServices(t *testing.T) {
	finder := &ServiceDiscoverer{
		AllowLocal: true,
		TimeLimit:  2 * time.Second,
	}

	broadcaster := &ServiceDiscoverer{
		BroadcastDelay: 10 * time.Millisecond,
		TimeLimit:      2 * time.Second,
	}

	go func() {
		ctx := context.Background()
		err := broadcaster.BroadcastService(ctx, []byte("test1"))
		assert.Nil(t, err)
	}()

	ctx := context.Background()
	services, err := finder.FindServices(ctx, []byte("test1"))
	assert.Nil(t, err)
	assert.NotZero(t, services)
	for _, s := range services {
		fmt.Println("Address:", s.Address)
		fmt.Println("Payload:", string(s.PayloadResponse))
		fmt.Println("")
	}
}
