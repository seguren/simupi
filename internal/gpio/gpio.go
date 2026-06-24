package gpio

import (
	"strconv"
	"sync"
	"time"
)

// GPIOManager manages the simulated state of GPIO input and output pins.
// All methods are safe for concurrent use.
type GPIOManager struct {
	mu      sync.RWMutex
	inputs  map[int]int
	outputs map[int]int
}

// NewGPIOManager creates a manager with the given pin numbers, all initialised to 0.
func NewGPIOManager(inputs, outputs []int) *GPIOManager {
	m := &GPIOManager{
		inputs:  make(map[int]int, len(inputs)),
		outputs: make(map[int]int, len(outputs)),
	}
	for _, p := range inputs {
		m.inputs[p] = 0
	}
	for _, p := range outputs {
		m.outputs[p] = 0
	}
	return m
}

func (m *GPIOManager) InputExists(pin int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.inputs[pin]
	return ok
}

func (m *GPIOManager) OutputExists(pin int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.outputs[pin]
	return ok
}

func (m *GPIOManager) SetInput(pin, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.inputs[pin]; ok {
		m.inputs[pin] = value
	}
}

func (m *GPIOManager) SetOutput(pin, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.outputs[pin]; ok {
		m.outputs[pin] = value
	}
}

// ToggleInput flips the value of an input pin and returns the new value.
func (m *GPIOManager) ToggleInput(pin int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := 1 - m.inputs[pin]
	m.inputs[pin] = v
	return v
}

// PulseInput sets a pin HIGH, waits duration, then sets it LOW.
// onComplete is called after the LOW transition (may be nil).
func (m *GPIOManager) PulseInput(pin int, duration time.Duration, onComplete func()) {
	m.SetInput(pin, 1)
	go func() {
		time.Sleep(duration)
		m.SetInput(pin, 0)
		if onComplete != nil {
			onComplete()
		}
	}()
}

// GetState returns a snapshot of all pin values.
// Keys are string pin numbers for JSON compatibility.
func (m *GPIOManager) GetState() (inputs, outputs map[string]int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inputs = make(map[string]int, len(m.inputs))
	outputs = make(map[string]int, len(m.outputs))
	for p, v := range m.inputs {
		inputs[strconv.Itoa(p)] = v
	}
	for p, v := range m.outputs {
		outputs[strconv.Itoa(p)] = v
	}
	return
}

// InputPins returns the configured input pin numbers.
func (m *GPIOManager) InputPins() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pins := make([]int, 0, len(m.inputs))
	for p := range m.inputs {
		pins = append(pins, p)
	}
	return pins
}

// OutputPins returns the configured output pin numbers.
func (m *GPIOManager) OutputPins() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pins := make([]int, 0, len(m.outputs))
	for p := range m.outputs {
		pins = append(pins, p)
	}
	return pins
}

// Restore sets pin values from a previously saved AppState.
func (m *GPIOManager) Restore(inputs, outputs map[string]int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range inputs {
		if pin, err := strconv.Atoi(k); err == nil {
			if _, ok := m.inputs[pin]; ok {
				m.inputs[pin] = v
			}
		}
	}
	for k, v := range outputs {
		if pin, err := strconv.Atoi(k); err == nil {
			if _, ok := m.outputs[pin]; ok {
				m.outputs[pin] = v
			}
		}
	}
}
