package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// NewTestRepo creates a temporary git repository with an initial commit.
// Returns the path to the repo. The repo is cleaned up when the test finishes.
func NewTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	repo := filepath.Join(dir, "test-repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v: %s: %v", args, out, err)
		}
	}

	// Create an initial commit
	readmePath := filepath.Join(repo, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cmds = [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %v: %s: %v", args, out, err)
		}
	}

	return repo
}

// CreateFile creates a file in the given directory with the given content.
func CreateFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// GitCmd runs a git command in the given directory.
func GitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return string(out)
}
