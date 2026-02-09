package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunPostCreateHooksSuccess(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(dir, []string{"touch marker.txt"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Error != nil {
		t.Errorf("expected no error, got %v", results[0].Error)
	}

	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); os.IsNotExist(err) {
		t.Error("expected marker.txt to exist")
	}
}

func TestRunPostCreateHooksPartialFailure(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(dir, []string{
		"touch good.txt",
		"false", // always fails
		"touch also-good.txt",
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Error != nil {
		t.Error("expected first hook to succeed")
	}
	if results[1].Error == nil {
		t.Error("expected second hook to fail")
	}
	if results[2].Error != nil {
		t.Error("expected third hook to succeed")
	}

	// Both good hooks should have run
	if _, err := os.Stat(filepath.Join(dir, "good.txt")); os.IsNotExist(err) {
		t.Error("expected good.txt to exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "also-good.txt")); os.IsNotExist(err) {
		t.Error("expected also-good.txt to exist")
	}
}

func TestRunPostCreateHooksEmpty(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(dir, nil)

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunPostCreateHooksShellFeatures(t *testing.T) {
	dir := t.TempDir()

	// Test shell features: pipes and quoting
	results := RunPostCreateHooks(dir, []string{
		"echo 'hello world' > output.txt",
	})

	if results[0].Error != nil {
		t.Fatalf("expected no error, got %v", results[0].Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", string(data))
	}
}
