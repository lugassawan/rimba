package observability

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Recorder binds one logical command invocation. A nil *Recorder no-ops on
// every method below — callers thread it through shared code paths with zero
// "if rec != nil" branching. Bind at the CLI's PersistentPreRunE (finalized in
// Execute()'s defer) or, for the long-lived MCP server, once per tool-call
// handler (finalized at handler return).
type Recorder struct {
	sink         Sink
	runID        string
	rootSpanID   string
	seq          atomic.Uint64 // lock-free monotonic sequence, independent of the sink's write mutex
	spanCounter  atomic.Uint64
	command      string
	task         string
	service      string
	rimbaVersion string
	debugTiming  bool // mirrors internal/debug's RIMBA_DEBUG check
	start        time.Time
}

// runIDCounter disambiguates run IDs generated within the same nanosecond by
// the same process.
var runIDCounter atomic.Uint64

// NewRecorder builds a Recorder bound to sink for one command invocation.
// Prefer Maybe over calling this directly — it's the single call composition
// roots should use.
func NewRecorder(sink Sink, command, task, service, rimbaVersion string) *Recorder {
	r := &Recorder{
		sink:         sink,
		runID:        newRunID(),
		command:      command,
		task:         task,
		service:      service,
		rimbaVersion: rimbaVersion,
		start:        time.Now(),
	}
	if _, ok := os.LookupEnv("RIMBA_DEBUG"); ok {
		r.debugTiming = true
	}
	r.rootSpanID = r.newSpanID()
	return r
}

// Maybe returns nil if observability is disabled or sink is nil, otherwise a
// fresh Recorder via NewRecorder. This is the single call composition roots
// use — they never construct Recorder{} directly.
func Maybe(enabled bool, sink Sink, command, task, service, rimbaVersion string) *Recorder {
	if !enabled || sink == nil {
		return nil
	}
	return NewRecorder(sink, command, task, service, rimbaVersion)
}

// LogSubprocess appends one SubprocessRecord to the log stream. failed marks
// the outcome; stderr is only persisted (truncated) when failed is true.
// Nil-safe.
func (r *Recorder) LogSubprocess(category, dir string, args []string, exitCode int, duration time.Duration, stderr string, failed bool) {
	if r == nil {
		return
	}
	outcome := OutcomeSuccess
	var recordedStderr string
	if failed {
		outcome = OutcomeError
		recordedStderr = truncate(stderr, stderrTruncateLimit)
	}

	rec := SubprocessRecord{
		SchemaVersion: SchemaVersion,
		Kind:          "subprocess",
		RunID:         r.runID,
		Seq:           r.seq.Add(1),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Category:      category,
		Args:          args,
		Dir:           dir,
		Outcome:       outcome,
		ExitCode:      exitCode,
		DurationMS:    duration.Milliseconds(),
		Stderr:        recordedStderr,
	}
	_ = r.sink.WriteLog(rec)

	if r.debugTiming {
		label := category + " " + strings.Join(args, " ")
		if dir != "" {
			label += " [" + filepath.Base(dir) + "]"
		}
		fmt.Fprintf(os.Stderr, "\n[debug] %s: %s\n", label, duration.Round(time.Millisecond))
	}
}

// LogError appends one ErrorRecord to the log stream. No-op if r or err is nil.
func (r *Recorder) LogError(context string, err error) {
	if r == nil || err == nil {
		return
	}
	rec := ErrorRecord{
		SchemaVersion: SchemaVersion,
		Kind:          "error",
		RunID:         r.runID,
		Seq:           r.seq.Add(1),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Command:       r.command,
		Message:       truncate(context+": "+err.Error(), stderrTruncateLimit),
	}
	_ = r.sink.WriteLog(rec)
}

// StartSpan starts a named child span under the command's root span and
// returns a function that records its duration when called. Nil-safe
// (returns a no-op closure).
func (r *Recorder) StartSpan(name string) func() {
	if r == nil {
		return func() {}
	}
	start := time.Now()
	spanID := r.newSpanID()
	return func() {
		r.writeSpan(spanID, r.rootSpanID, name, time.Since(start), "")
	}
}

// StartModuleSpan is StartSpan specialized for a dependency-module install,
// naming the span "deps:<filepath.Base(dir)>" and recording whether the
// module was cloned from a sibling worktree or freshly installed. Nil-safe.
func (r *Recorder) StartModuleSpan(dir string) func(cloned bool) {
	if r == nil {
		return func(bool) {}
	}
	start := time.Now()
	spanID := r.newSpanID()
	name := "deps:" + filepath.Base(dir)
	return func(cloned bool) {
		detail := "installed"
		if cloned {
			detail = "cloned"
		}
		r.writeSpan(spanID, r.rootSpanID, name, time.Since(start), detail)
	}
}

// Finalize writes the CommandRecord (log stream) and the root SpanRecord
// (metrics stream) covering the whole invocation. Call exactly once, after
// the command's outcome is known. Nil-safe.
func (r *Recorder) Finalize(outcome string, exitCode int, err error) {
	if r == nil {
		return
	}
	d := time.Since(r.start)
	rec := CommandRecord{
		SchemaVersion: SchemaVersion,
		Kind:          "command",
		RunID:         r.runID,
		Seq:           r.seq.Add(1),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		RimbaVersion:  r.rimbaVersion,
		Command:       r.command,
		Task:          r.task,
		Service:       r.service,
		Outcome:       outcome,
		ExitCode:      exitCode,
		DurationMS:    d.Milliseconds(),
		Error:         truncate(errString(err), stderrTruncateLimit),
	}
	_ = r.sink.WriteLog(rec)
	r.writeSpan(r.rootSpanID, "", "command", d, "")
}

// Close releases the underlying sink's file handles. Nil-safe. Callers defer
// this immediately after Maybe(...) returns; Finalize must run before Close
// (see cmd/root.go's Execute and internal/mcp/observability.go's withRecorder).
func (r *Recorder) Close() error {
	if r == nil {
		return nil
	}
	return r.sink.Close()
}

// newRunID returns a run identifier unique enough for local correlation:
// timestamp+pid+counter, no UUID dependency.
func newRunID() string {
	return fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), runIDCounter.Add(1))
}

func (r *Recorder) newSpanID() string {
	return fmt.Sprintf("%s-%d", r.runID, r.spanCounter.Add(1))
}

// writeSpan builds and writes one SpanRecord to the metrics stream. Shared by
// StartSpan, StartModuleSpan, and Finalize's root-span write.
func (r *Recorder) writeSpan(spanID, parentSpanID, name string, d time.Duration, detail string) {
	rec := SpanRecord{
		SchemaVersion: SchemaVersion,
		Kind:          "span",
		RunID:         r.runID,
		SpanID:        spanID,
		ParentSpanID:  parentSpanID,
		Seq:           r.seq.Add(1),
		Timestamp:     time.Now().Format(time.RFC3339Nano),
		Command:       r.command,
		Name:          name,
		DurationMS:    d.Milliseconds(),
		Detail:        detail,
	}
	_ = r.sink.WriteMetric(rec)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
