package fleet

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	state := &TaskState{
		Name:      "test-task",
		Agent:     "claude",
		Branch:    "feature/test-task",
		Path:      "/worktrees/feature/test-task",
		PID:       12345,
		Status:    StatusRunning,
		StartedAt: time.Now().Truncate(time.Millisecond),
		LogFile:   "/logs/test-task.log",
	}

	if err := SaveState(dir, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(dir, "test-task")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if loaded.Name != state.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, state.Name)
	}
	if loaded.Agent != state.Agent {
		t.Errorf("Agent = %q, want %q", loaded.Agent, state.Agent)
	}
	if loaded.Status != state.Status {
		t.Errorf("Status = %q, want %q", loaded.Status, state.Status)
	}
	if loaded.PID != state.PID {
		t.Errorf("PID = %d, want %d", loaded.PID, state.PID)
	}
}

func TestSaveStateCreatesDirs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	state := &TaskState{Name: "task", Status: StatusComplete}

	if err := SaveState(dir, state); err != nil {
		t.Fatalf("SaveState should create dirs: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "task.json")); err != nil {
		t.Errorf("state file should exist: %v", err)
	}
}

func TestLoadStateNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadState(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing state file")
	}
}

func TestLoadStateInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadState(dir, "bad")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadAllStates(t *testing.T) {
	dir := t.TempDir()

	tasks := []*TaskState{
		{Name: "alpha", Status: StatusRunning},
		{Name: "beta", Status: StatusComplete},
	}
	for _, s := range tasks {
		if err := SaveState(dir, s); err != nil {
			t.Fatal(err)
		}
	}

	// Add a non-JSON file that should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("skip"), 0600); err != nil {
		t.Fatal(err)
	}

	// Add an invalid JSON file that should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}

	states, err := LoadAllStates(dir)
	if err != nil {
		t.Fatalf("LoadAllStates: %v", err)
	}
	if len(states) != 2 {
		t.Errorf("got %d states, want 2", len(states))
	}
}

func TestLoadAllStatesNonexistentDir(t *testing.T) {
	states, err := LoadAllStates("/nonexistent/dir")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got: %v", err)
	}
	if states != nil {
		t.Errorf("expected nil states, got %v", states)
	}
}

func TestLoadAllStatesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	states, err := LoadAllStates(dir)
	if err != nil {
		t.Fatalf("LoadAllStates: %v", err)
	}
	if len(states) != 0 {
		t.Errorf("expected 0 states, got %d", len(states))
	}
}

func TestRemoveState(t *testing.T) {
	dir := t.TempDir()
	state := &TaskState{Name: "removeme", Status: StatusStopped}
	if err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	if err := RemoveState(dir, "removeme"); err != nil {
		t.Fatalf("RemoveState: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "removeme.json")); !os.IsNotExist(err) {
		t.Error("state file should have been removed")
	}
}

func TestRemoveStateNotFound(t *testing.T) {
	dir := t.TempDir()
	err := RemoveState(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error removing nonexistent state")
	}
}

func TestTaskStatusValues(t *testing.T) {
	tests := []struct {
		status TaskStatus
		want   string
	}{
		{StatusRunning, "running"},
		{StatusStopped, "stopped"},
		{StatusComplete, "complete"},
		{StatusFailed, "failed"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("status = %q, want %q", tt.status, tt.want)
		}
	}
}
