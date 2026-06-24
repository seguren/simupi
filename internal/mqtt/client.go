package mqtt

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// OutputSetter is the subset of GPIOManager needed by the client.
type OutputSetter interface {
	SetOutput(pin int, value int)
	OutputExists(pin int) bool
}

// Client publishes sensor and GPIO input values, and subscribes to GPIO output commands.
type Client struct {
	inner paho.Client

	mu   sync.RWMutex
	gpio OutputSetter
}

// NewClient creates a client that connects to brokerAddr (e.g. "tcp://localhost:1883").
func NewClient(brokerAddr string, gpio OutputSetter) *Client {
	c := &Client{gpio: gpio}

	opts := paho.NewClientOptions()
	opts.AddBroker(brokerAddr)
	opts.SetClientID("simupi")
	opts.SetAutoReconnect(true)
	opts.SetOnConnectHandler(c.onConnect)
	opts.SetConnectionLostHandler(func(_ paho.Client, _ error) {})

	c.inner = paho.NewClient(opts)
	return c
}

// Start connects to the broker, retrying up to 10 times.
func (c *Client) Start() error {
	var lastErr error
	for i := 0; i < 10; i++ {
		token := c.inner.Connect()
		if token.WaitTimeout(2*time.Second) && token.Error() == nil {
			return nil
		}
		lastErr = token.Error()
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("mqtt client: connect failed: %w", lastErr)
}

// Stop disconnects cleanly.
func (c *Client) Stop() {
	c.inner.Disconnect(500)
}

// SetGPIO swaps the GPIO reference on scenario reload.
func (c *Client) SetGPIO(gpio OutputSetter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gpio = gpio
}

func (c *Client) onConnect(cl paho.Client) {
	cl.Subscribe("gpio/out/#", 0, c.handleGPIOOut)
}

func (c *Client) handleGPIOOut(_ paho.Client, msg paho.Message) {
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) != 3 {
		return
	}
	pin, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	c.mu.RLock()
	gpio := c.gpio
	c.mu.RUnlock()

	if gpio == nil || !gpio.OutputExists(pin) {
		return
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(msg.Payload())))
	if err != nil {
		return
	}
	gpio.SetOutput(pin, value)
}

// PublishSensor publishes a sensor reading to sensor/<name>.
func (c *Client) PublishSensor(name string, value float64) {
	payload := strconv.FormatFloat(value, 'f', 2, 64)
	c.inner.Publish("sensor/"+name, 0, false, payload)
}

// PublishInput publishes a GPIO input reading to gpio/in/<pin>.
func (c *Client) PublishInput(pin int, value int) {
	c.inner.Publish(fmt.Sprintf("gpio/in/%d", pin), 0, false, strconv.Itoa(value))
}
