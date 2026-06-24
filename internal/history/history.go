package history

import (
	"sync"
	"time"
)

const maxEntries = 500

// Entry is a single timestamped sensor reading.
type Entry struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// HistoryManager keeps a circular buffer of readings per sensor.
// All methods are safe for concurrent use.
type HistoryManager struct {
	mu      sync.RWMutex
	entries map[string][]Entry
}

func NewHistoryManager() *HistoryManager {
	return &HistoryManager{
		entries: make(map[string][]Entry),
	}
}

// Add appends a reading for the given sensor, evicting the oldest entry
// once the buffer exceeds maxEntries.
func (m *HistoryManager) Add(sensor string, value float64) {
	entry := Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Value:     value,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	buf := append(m.entries[sensor], entry)
	if len(buf) > maxEntries {
		buf = buf[len(buf)-maxEntries:]
	}
	m.entries[sensor] = buf
}

// Get returns a copy of the history for a single sensor.
func (m *HistoryManager) Get(sensor string) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.entries[sensor]
	out := make([]Entry, len(src))
	copy(out, src)
	return out
}

// GetAll returns a copy of the history for every sensor.
func (m *HistoryManager) GetAll() map[string][]Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]Entry, len(m.entries))
	for sensor, src := range m.entries {
		buf := make([]Entry, len(src))
		copy(buf, src)
		out[sensor] = buf
	}
	return out
}

// Clear removes all history (called on scenario reload).
func (m *HistoryManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string][]Entry)
}
