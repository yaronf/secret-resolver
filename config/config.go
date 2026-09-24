package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// File is the resolver process configuration.
type File struct {
	Providers []Provider `json:"providers"`
}

// Provider describes one provider child process.
type Provider struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// LoadJSON reads a JSON config file.
func LoadJSON(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if len(f.Providers) == 0 {
		return nil, fmt.Errorf("config: no providers")
	}
	for i, p := range f.Providers {
		if p.Command == "" {
			return nil, fmt.Errorf("config: providers[%d]: empty command", i)
		}
	}
	return &f, nil
}
