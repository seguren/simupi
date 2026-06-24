package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const stateFile = "data/state.json"

// SaveState writes the application runtime state to disk atomically.
func SaveState(s *AppState) error {
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := stateFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, stateFile)
}

// LoadState reads the persisted state from disk.
// Returns nil, nil on first run (no state file yet).
func LoadState() (*AppState, error) {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s AppState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
