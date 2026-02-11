package fleet

import (
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// Manifest represents a fleet.toml file.
type Manifest struct {
	Tasks []TaskSpec `toml:"tasks"`
}

// TaskSpec defines a single task in the fleet manifest.
type TaskSpec struct {
	Name   string `toml:"name"`
	Type   string `toml:"type,omitempty"`
	Agent  string `toml:"agent,omitempty"`
	Prompt string `toml:"prompt,omitempty"`
}

// LoadManifest loads a fleet manifest from a TOML file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest not found: %w", err)
	}

	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	for i, t := range m.Tasks {
		if t.Name == "" {
			return nil, fmt.Errorf("task[%d]: name is required", i)
		}
	}

	return &m, nil
}

// SaveManifest writes a fleet manifest to a TOML file.
func SaveManifest(path string, m *Manifest) error {
	data, err := toml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}
