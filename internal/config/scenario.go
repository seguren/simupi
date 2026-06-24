package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const scenariosDataDir = "data/scenarios"

// ScenarioManager loads and saves simulation scenarios.
// Embedded scenarios are read-only defaults; user scenarios live in data/scenarios/
// and shadow embedded ones of the same name.
type ScenarioManager struct {
	fs embed.FS
}

func NewScenarioManager(fs embed.FS) *ScenarioManager {
	return &ScenarioManager{fs: fs}
}

// List returns all available scenario names sorted alphabetically.
func (m *ScenarioManager) List() []string {
	seen := map[string]struct{}{}

	if entries, err := m.fs.ReadDir("scenarios"); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				seen[strings.TrimSuffix(e.Name(), ".json")] = struct{}{}
			}
		}
	}

	if entries, err := os.ReadDir(scenariosDataDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				seen[strings.TrimSuffix(e.Name(), ".json")] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Load returns the named scenario, preferring user-saved over embedded.
func (m *ScenarioManager) Load(name string) (*Scenario, error) {
	var data []byte
	var err error

	data, err = os.ReadFile(filepath.Join(scenariosDataDir, name+".json"))
	if err != nil {
		data, err = m.fs.ReadFile("scenarios/" + name + ".json")
		if err != nil {
			return nil, fmt.Errorf("scenario %q not found", name)
		}
	}

	var s Scenario
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("scenario %q: %w", name, err)
	}
	if s.Name == "" {
		s.Name = name
	}
	if s.Aliases == nil {
		s.Aliases = map[string]string{}
	}
	if s.Inputs == nil {
		s.Inputs = []int{}
	}
	if s.Outputs == nil {
		s.Outputs = []int{}
	}
	return &s, nil
}

// Save writes a scenario to the data directory atomically.
func (m *ScenarioManager) Save(name string, s *Scenario) error {
	if err := os.MkdirAll(scenariosDataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(scenariosDataDir, name+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
