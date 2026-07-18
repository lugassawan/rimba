package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func stubMachine(t *testing.T, info MachineInfo) {
	t.Helper()
	orig := currentMachine
	currentMachine = func() MachineInfo { return info }
	t.Cleanup(func() { currentMachine = orig })
}

func readFlushedLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return lines
}

func TestNewRecorderNeverReturnsNil(t *testing.T) {
	r := NewRecorder("add", "task-1", "svc")
	if r == nil {
		t.Fatal("NewRecorder returned nil")
	}
}

func TestMaybe(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		wantNil bool
	}{
		{name: "disabled returns nil", enabled: false, wantNil: true},
		{name: "enabled returns non-nil", enabled: true, wantNil: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Maybe(tt.enabled, "add", "task-1", "svc")
			if tt.wantNil && r != nil {
				t.Error("expected nil recorder")
			}
			if !tt.wantNil && r == nil {
				t.Error("expected non-nil recorder")
			}
		})
	}
}

func TestStartSpanRecordsDurationAndName(t *testing.T) {
	r := NewRecorder("add", "task-1", "svc")

	stop := r.StartSpan("copy")
	stop()

	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := r.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	lines := readFlushedLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var run Run
	if err := json.Unmarshal([]byte(lines[0]), &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(run.Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(run.Spans))
	}
	if run.Spans[0].Name != "copy" {
		t.Errorf("span name = %q, want %q", run.Spans[0].Name, "copy")
	}
	if run.Spans[0].DurationMS < 0 {
		t.Errorf("duration_ms = %d, want >= 0", run.Spans[0].DurationMS)
	}
}

func TestStartModuleSpanRecordsClonedFlag(t *testing.T) {
	tests := []struct {
		name   string
		cloned bool
	}{
		{name: "cloned true", cloned: true},
		{name: "cloned false", cloned: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRecorder("add", "task-1", "svc")
			stop := r.StartModuleSpan("modules/foo")
			stop(tt.cloned)

			path := filepath.Join(t.TempDir(), "metrics.jsonl")
			if err := r.Flush(path, 0); err != nil {
				t.Fatalf("Flush: %v", err)
			}

			var run Run
			lines := readFlushedLines(t, path)
			if err := json.Unmarshal([]byte(lines[0]), &run); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if len(run.Spans) != 1 {
				t.Fatalf("expected 1 span, got %d", len(run.Spans))
			}
			if run.Spans[0].Cloned == nil {
				t.Fatal("expected Cloned to be set")
			}
			if *run.Spans[0].Cloned != tt.cloned {
				t.Errorf("Cloned = %v, want %v", *run.Spans[0].Cloned, tt.cloned)
			}
		})
	}
}

func TestConcurrentSpanRecording(t *testing.T) {
	r := NewRecorder("add", "task-1", "svc")

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			stop := r.StartSpan("deps")
			stop()
		}()
	}
	wg.Wait()

	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := r.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var run Run
	lines := readFlushedLines(t, path)
	if err := json.Unmarshal([]byte(lines[0]), &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(run.Spans) != n {
		t.Fatalf("expected %d spans, got %d", n, len(run.Spans))
	}
}

func TestFlushMissingFileCreatesItWithOneLine(t *testing.T) {
	r := NewRecorder("add", "task-1", "svc")
	path := filepath.Join(t.TempDir(), "sub", "metrics.jsonl")

	if err := r.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	lines := readFlushedLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestFlushAppendsAndTrims(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.jsonl")

	const maxRuns = 3
	const totalFlushes = 5

	for i := range totalFlushes {
		r := NewRecorder("add", "task-1", "svc")
		stop := r.StartSpan(fmt.Sprintf("span-%d", i))
		stop()
		if err := r.Flush(path, maxRuns); err != nil {
			t.Fatalf("Flush %d: %v", i, err)
		}
	}

	lines := readFlushedLines(t, path)
	if len(lines) != maxRuns {
		t.Fatalf("expected %d lines, got %d", maxRuns, len(lines))
	}

	// The survivors must be the LAST maxRuns iterations, oldest-first,
	// newest-last — i.e. span-2, span-3, span-4 in that order.
	wantSpanNames := make([]string, maxRuns)
	for i := range maxRuns {
		wantSpanNames[i] = fmt.Sprintf("span-%d", totalFlushes-maxRuns+i)
	}

	for i, line := range lines {
		var run Run
		if err := json.Unmarshal([]byte(line), &run); err != nil {
			t.Fatalf("unmarshal line %d: %v", i, err)
		}
		if len(run.Spans) != 1 {
			t.Fatalf("line %d: expected 1 span, got %d", i, len(run.Spans))
		}
		if run.Spans[0].Name != wantSpanNames[i] {
			t.Errorf("line %d: span name = %q, want %q", i, run.Spans[0].Name, wantSpanNames[i])
		}
	}
}

func TestFlushSpanAccumulationOrder(t *testing.T) {
	r := NewRecorder("add", "task-1", "svc")

	stopA := r.StartSpan("a")
	stopA()
	stopB := r.StartSpan("b")
	stopB()

	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := r.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var run Run
	lines := readFlushedLines(t, path)
	if err := json.Unmarshal([]byte(lines[0]), &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(run.Spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(run.Spans))
	}
	if run.Spans[0].Name != "a" {
		t.Errorf("Spans[0].Name = %q, want %q", run.Spans[0].Name, "a")
	}
	if run.Spans[1].Name != "b" {
		t.Errorf("Spans[1].Name = %q, want %q", run.Spans[1].Name, "b")
	}
}

func TestFlushMaxRunsLEZeroUsesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.jsonl")

	tests := []int{0, -1, -100}
	for _, maxRuns := range tests {
		r := NewRecorder("add", "task-1", "svc")
		if err := r.Flush(path, maxRuns); err != nil {
			t.Fatalf("Flush maxRuns=%d: %v", maxRuns, err)
		}
	}

	lines := readFlushedLines(t, path)
	if len(lines) != len(tests) {
		t.Fatalf("expected %d lines (well under DefaultMaxRuns), got %d", len(tests), len(lines))
	}
	if DefaultMaxRuns <= len(tests) {
		t.Fatalf("test assumption broken: DefaultMaxRuns=%d must exceed %d", DefaultMaxRuns, len(tests))
	}
}

func TestNilRecorderMethodsNoOp(t *testing.T) {
	var r *Recorder

	stop := r.StartSpan("copy")
	if stop == nil {
		t.Fatal("StartSpan on nil recorder returned nil stop func")
	}
	stop()

	moduleStop := r.StartModuleSpan("modules/foo")
	if moduleStop == nil {
		t.Fatal("StartModuleSpan on nil recorder returned nil stop func")
	}
	moduleStop(true)

	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := r.Flush(path, 0); err != nil {
		t.Errorf("Flush on nil recorder returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Flush on nil recorder should not create a file, stat err = %v", err)
	}
}

func TestMachineInfoCaptureSeam(t *testing.T) {
	stubMachine(t, MachineInfo{OS: "stuboS", Arch: "stubArch", NumCPU: 7})

	r := NewRecorder("add", "task-1", "svc")
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := r.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var run Run
	lines := readFlushedLines(t, path)
	if err := json.Unmarshal([]byte(lines[0]), &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := MachineInfo{OS: "stuboS", Arch: "stubArch", NumCPU: 7}
	if run.Machine != want {
		t.Errorf("Machine = %+v, want %+v", run.Machine, want)
	}
}

func TestRunSchemaFields(t *testing.T) {
	r := NewRecorder("add", "task-1", "svc")
	path := filepath.Join(t.TempDir(), "metrics.jsonl")
	if err := r.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var run Run
	lines := readFlushedLines(t, path)
	if err := json.Unmarshal([]byte(lines[0]), &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if run.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", run.SchemaVersion, SchemaVersion)
	}
	if run.Command != "add" {
		t.Errorf("Command = %q, want %q", run.Command, "add")
	}
	if run.Task != "task-1" {
		t.Errorf("Task = %q, want %q", run.Task, "task-1")
	}
	if run.Service != "svc" {
		t.Errorf("Service = %q, want %q", run.Service, "svc")
	}
	if run.DurationMS < 0 {
		t.Errorf("DurationMS = %d, want >= 0", run.DurationMS)
	}
	if _, err := time.Parse(time.RFC3339, run.Timestamp); err != nil {
		t.Errorf("Timestamp %q not RFC3339: %v", run.Timestamp, err)
	}
}

func TestFlushExistingEmptyFileTreatedAsNoLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.jsonl")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	r := NewRecorder("add", "task-1", "svc")
	if err := r.Flush(path, 0); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	lines := readFlushedLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
}

func TestFlushMkdirAllFailsWhenParentIsAFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}

	r := NewRecorder("add", "task-1", "svc")
	path := filepath.Join(blocker, "metrics.jsonl") // blocker is a file, not a dir

	if err := r.Flush(path, 0); err == nil {
		t.Fatal("expected error when parent path component is a regular file")
	}
}

func TestFlushReadExistingFileErrorWhenPathIsADirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.jsonl")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("setup Mkdir: %v", err)
	}

	r := NewRecorder("add", "task-1", "svc")
	if err := r.Flush(path, 0); err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestFlushWriteFileFailsOnReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not block writes")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("setup Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	r := NewRecorder("add", "task-1", "svc")
	path := filepath.Join(dir, "metrics.jsonl")

	if err := r.Flush(path, 0); err == nil {
		t.Fatal("expected error writing to a read-only directory")
	}
}
