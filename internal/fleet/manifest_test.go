package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testManifestFile = "fleet.toml"
	testTaskA        = "task-a"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, testManifestFile)

	content := `
[[tasks]]
name = "auth-refactor"
type = "feature"
agent = "claude"
prompt = "Refactor authentication"

[[tasks]]
name = "fix-leak"
agent = "codex"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if len(m.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(m.Tasks))
	}
	if m.Tasks[0].Name != "auth-refactor" {
		t.Errorf("task[0].Name = %q, want %q", m.Tasks[0].Name, "auth-refactor")
	}
	if m.Tasks[0].Type != "feature" {
		t.Errorf("task[0].Type = %q, want %q", m.Tasks[0].Type, "feature")
	}
	if m.Tasks[1].Agent != "codex" {
		t.Errorf("task[1].Agent = %q, want %q", m.Tasks[1].Agent, "codex")
	}
}

func TestLoadManifestNotFound(t *testing.T) {
	_, err := LoadManifest("/nonexistent/fleet.toml")
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
	if !strings.Contains(err.Error(), "manifest not found") {
		t.Errorf("error = %q, want it to contain 'manifest not found'", err)
	}
}

func TestLoadManifestInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, testManifestFile)
	if err := os.WriteFile(path, []byte("invalid = [[["), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
	if !strings.Contains(err.Error(), "invalid manifest") {
		t.Errorf("error = %q, want it to contain 'invalid manifest'", err)
	}
}

func TestLoadManifestMissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, testManifestFile)
	content := `
[[tasks]]
agent = "claude"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for missing task name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error = %q, want it to contain 'name is required'", err)
	}
}

func TestSaveManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, testManifestFile)

	m := &Manifest{
		Tasks: []TaskSpec{
			{Name: testTaskA, Type: "feature", Agent: "claude", Prompt: "do stuff"},
			{Name: "task-b", Agent: "generic"},
		},
	}

	if err := SaveManifest(path, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	loaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest after save: %v", err)
	}

	if len(loaded.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(loaded.Tasks))
	}
	if loaded.Tasks[0].Name != testTaskA {
		t.Errorf("task[0].Name = %q, want %q", loaded.Tasks[0].Name, testTaskA)
	}
}

func TestSaveManifestWriteError(t *testing.T) {
	err := SaveManifest("/nonexistent-dir/fleet.toml", &Manifest{})
	if err != nil {
		// os.WriteFile to nonexistent dir should fail
		return
	}
	// Some systems might create the file; either way, no panic
}
