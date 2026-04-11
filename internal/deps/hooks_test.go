package deps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPostCreateHooksSuccess(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(dir, []string{"touch marker.txt"}, nil)

	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}

	if results[0].Error != nil {
		t.Errorf(fmtExpectedNoError, results[0].Error)
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
	}, nil)

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

	results := RunPostCreateHooks(dir, nil, nil)

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunPostCreateHooksShellFeatures(t *testing.T) {
	dir := t.TempDir()

	// Test shell features: pipes and quoting
	results := RunPostCreateHooks(dir, []string{
		"echo 'hello world' > output.txt",
	}, nil)

	if results[0].Error != nil {
		t.Fatalf(fmtExpectedNoError, results[0].Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", string(data))
	}
}

func TestRunPostCreateHooksProgressCallback(t *testing.T) {
	dir := t.TempDir()

	var calls []progressCall
	onProgress := func(msg string) {
		calls = append(calls, progressCall{msg})
	}

	hooks := []string{"touch a.txt", "touch b.txt"}
	RunPostCreateHooks(dir, hooks, onProgress)

	if len(calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(calls))
	}
	if want := "touch a.txt (1/2)"; calls[0].message != want {
		t.Errorf("calls[0] = %q, want %q", calls[0].message, want)
	}
	if want := "touch b.txt (2/2)"; calls[1].message != want {
		t.Errorf("calls[1] = %q, want %q", calls[1].message, want)
	}
}

func TestRunPostCreateHooksOutputCapture(t *testing.T) {
	dir := t.TempDir()

	results := RunPostCreateHooks(dir, []string{
		"echo hook-output-captured && exit 1",
	}, nil)

	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected error from failing hook")
	}
	if !strings.Contains(results[0].Error.Error(), "hook-output-captured") {
		t.Errorf("error should contain captured output, got %q", results[0].Error.Error())
	}
}
