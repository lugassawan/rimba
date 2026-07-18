package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Recorder accumulates spans for one command invocation. A nil *Recorder is
// safe to call every method on (all become no-ops) — see Global Constraints.
type Recorder struct {
	mu      sync.Mutex
	command string
	task    string
	service string
	start   time.Time
	spans   []Span
}

// NewRecorder starts a recorder for one command invocation. Never returns nil.
func NewRecorder(command, task, service string) *Recorder {
	return &Recorder{
		command: command,
		task:    task,
		service: service,
		start:   time.Now(),
	}
}

// Maybe returns NewRecorder(...) if enabled, else nil. Callers pass the
// result straight through the pipeline without an enabled/disabled branch
// anywhere else — nil-safety on Recorder's methods handles the rest.
func Maybe(enabled bool, command, task, service string) *Recorder {
	if !enabled {
		return nil
	}
	return NewRecorder(command, task, service)
}

// StartSpan starts a named span and returns a stop function that records its
// duration when called. Concurrency-safe: safe to start/stop many spans from
// different goroutines concurrently (deps installs run in parallel).
func (r *Recorder) StartSpan(name string) func() {
	if r == nil {
		return func() {}
	}
	start := time.Now()
	return func() {
		r.appendSpan(Span{Name: name, DurationMS: time.Since(start).Milliseconds()})
	}
}

// StartModuleSpan is like StartSpan but its stop function takes a `cloned`
// bool (true = CoW clone-from-sibling hit, false = ran the real install).
func (r *Recorder) StartModuleSpan(dir string) func(cloned bool) {
	if r == nil {
		return func(bool) {}
	}
	start := time.Now()
	return func(cloned bool) {
		r.appendSpan(Span{
			Name:       dir,
			DurationMS: time.Since(start).Milliseconds(),
			Cloned:     &cloned,
		})
	}
}

// Flush appends one JSON line for this run to path (creating it and its
// parent dir if needed), then trims the file to at most the last maxRuns
// lines (maxRuns <= 0 uses DefaultMaxRuns). Returns nil on a nil Recorder.
func (r *Recorder) Flush(path string, maxRuns int) error {
	if r == nil {
		return nil
	}
	if maxRuns <= 0 {
		maxRuns = DefaultMaxRuns
	}

	data, err := json.Marshal(r.snapshot())
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	lines = append(lines, string(data))
	if len(lines) > maxRuns {
		lines = lines[len(lines)-maxRuns:]
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// appendSpan records one completed span under the recorder's mutex.
func (r *Recorder) appendSpan(s Span) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, s)
}

// snapshot builds the Run record for this invocation as of now: duration and
// machine info are computed at flush time, not at NewRecorder time.
func (r *Recorder) snapshot() Run {
	r.mu.Lock()
	defer r.mu.Unlock()

	spans := make([]Span, len(r.spans))
	copy(spans, r.spans)

	return Run{
		SchemaVersion: SchemaVersion,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Command:       r.command,
		Task:          r.task,
		Service:       r.service,
		DurationMS:    time.Since(r.start).Milliseconds(),
		Machine:       currentMachine(),
		Spans:         spans,
	}
}

// readLines reads the non-empty lines of an existing JSONL file at path,
// treating a missing file as empty.
func readLines(path string) ([]string, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	trimmed := strings.TrimRight(string(existing), "\n")
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
