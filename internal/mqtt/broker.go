package mqtt

import (
	"fmt"
	"log"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// Broker wraps an embedded mochi-mqtt server.
type Broker struct {
	server *mochi.Server
}

// NewBroker creates a broker that will listen on the given TCP port.
func NewBroker(port int) (*Broker, error) {
	server := mochi.New(nil)

	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		return nil, fmt.Errorf("mqtt broker: add auth hook: %w", err)
	}

	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp1",
		Address: fmt.Sprintf(":%d", port),
	})
	if err := server.AddListener(tcp); err != nil {
		return nil, fmt.Errorf("mqtt broker: add listener: %w", err)
	}

	return &Broker{server: server}, nil
}

// Start launches the broker in a background goroutine.
func (b *Broker) Start() {
	go func() {
		if err := b.server.Serve(); err != nil {
			log.Printf("mqtt broker stopped: %v", err)
		}
	}()
}

// Stop shuts down the broker gracefully.
func (b *Broker) Stop() {
	if err := b.server.Close(); err != nil {
		log.Printf("mqtt broker close: %v", err)
	}
}
