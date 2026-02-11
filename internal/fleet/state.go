package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TaskStatus represents the status of a fleet task.
type TaskStatus string

const (
	StatusRunning  TaskStatus = "running"
	StatusStopped  TaskStatus = "stopped"
	StatusComplete TaskStatus = "complete"
	StatusFailed   TaskStatus = "failed"
)

// TaskState is the persisted state for a fleet task.
type TaskState struct {
	Name      string     `json:"name"`
	Agent     string     `json:"agent"`
	Branch    string     `json:"branch"`
	Path      string     `json:"path"`
	PID       int        `json:"pid"`
	Status    TaskStatus `json:"status"`
	StartedAt time.Time  `json:"started_at"`
	LogFile   string     `json:"log_file"`
}

// SaveState writes task state to a JSON file.
func SaveState(stateDir string, state *TaskState) error {
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return err
	}
	path := filepath.Join(stateDir, state.Name+".json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadState reads task state from a JSON file.
func LoadState(stateDir, taskName string) (*TaskState, error) {
	path := filepath.Join(stateDir, taskName+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("state not found for %q: %w", taskName, err)
	}

	var state TaskState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid state for %q: %w", taskName, err)
	}
	return &state, nil
}

// LoadAllStates reads all task state files from the state directory.
func LoadAllStates(stateDir string) ([]*TaskState, error) {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var states []*TaskState
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()[:len(e.Name())-len(".json")]
		state, err := LoadState(stateDir, name)
		if err != nil {
			continue // skip invalid state files
		}
		states = append(states, state)
	}
	return states, nil
}

// RemoveState deletes the state file for a task.
func RemoveState(stateDir, taskName string) error {
	path := filepath.Join(stateDir, taskName+".json")
	return os.Remove(path)
}
