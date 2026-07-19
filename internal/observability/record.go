package observability

// SchemaVersion is the current record schema version written by this build.
const SchemaVersion = 1

// Outcome values shared by CommandRecord and SubprocessRecord.
const (
	OutcomeSuccess = "success"
	OutcomeError   = "error"
)

// Subprocess categories.
const (
	CategoryGit  = "git"
	CategoryExec = "exec"
	CategoryHook = "hook"
	CategoryGH   = "gh"
)

// Module-span detail values (see SpanRecord.Detail). DetailClonedReflink and
// DetailClonedCopy split what used to be a single ambiguous "cloned" value —
// that ambiguity hid a real regression where a "clone" was actually a silent
// 14-123s byte-copy fallback in disguise as a sub-second reflink.
const (
	DetailInstalled     = "installed"
	DetailClonedReflink = "cloned-reflink"
	DetailClonedCopy    = "cloned-copy"
	DetailDeferred      = "deferred"
)

// stderrTruncateLimit caps captured stderr so day-file lines stay a bounded
// size. Cross-process append safety comes from a single write() syscall being
// atomic for regular files (not from PIPE_BUF, which governs pipes, not the
// O_APPEND regular-file writes used here) — this cap is about keeping lines
// reasonably sized, not about atomicity.
const stderrTruncateLimit = 2 * 1024

// CommandRecord is written once per command invocation, at Finalize, to the
// .log.jsonl stream.
type CommandRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"` // always "command"
	RunID         string `json:"run_id"`
	Seq           uint64 `json:"seq"`
	Timestamp     string `json:"timestamp"` // RFC3339Nano
	RimbaVersion  string `json:"rimba_version"`
	Command       string `json:"command"`
	Task          string `json:"task,omitempty"`
	Service       string `json:"service,omitempty"`
	Outcome       string `json:"outcome"`
	ExitCode      int    `json:"exit_code"`
	DurationMS    int64  `json:"duration_ms"`
	Error         string `json:"error,omitempty"`
}

// SubprocessRecord is written once per git/gh/exec/hook subprocess to the .log.jsonl stream.
type SubprocessRecord struct {
	SchemaVersion int      `json:"schema_version"`
	Kind          string   `json:"kind"` // always "subprocess"
	RunID         string   `json:"run_id"`
	Seq           uint64   `json:"seq"`
	Timestamp     string   `json:"timestamp"`
	Category      string   `json:"category"` // CategoryGit | CategoryGH | CategoryExec | CategoryHook
	Args          []string `json:"args,omitempty"`
	Dir           string   `json:"dir,omitempty"`
	Outcome       string   `json:"outcome"`
	ExitCode      int      `json:"exit_code"`
	DurationMS    int64    `json:"duration_ms"`
	Stderr        string   `json:"stderr,omitempty"` // only populated when Outcome == OutcomeError; truncated
}

// ErrorRecord is written to the .log.jsonl stream for errors not already
// captured by a CommandRecord/SubprocessRecord outcome.
type ErrorRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"` // always "error"
	RunID         string `json:"run_id"`
	Seq           uint64 `json:"seq"`
	Timestamp     string `json:"timestamp"`
	Command       string `json:"command"`
	Message       string `json:"message"`
}

// SpanRecord is written to the compact .metrics.jsonl stream — one root span
// per command (Name == "command", ParentSpanID empty) plus one per nested
// phase/module span (ParentSpanID == the root's SpanID). This is the whole
// "run" concept: a command's root span IS its run record.
type SpanRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"` // always "span"
	RunID         string `json:"run_id"`
	SpanID        string `json:"span_id"`
	ParentSpanID  string `json:"parent_span_id,omitempty"`
	Seq           uint64 `json:"seq"`
	Timestamp     string `json:"timestamp"`
	Command       string `json:"command"`
	Name          string `json:"name"` // "command" for the root span, else phase/module name
	DurationMS    int64  `json:"duration_ms"`
	Detail        string `json:"detail,omitempty"` // e.g. DetailClonedReflink / DetailClonedCopy / DetailInstalled for module spans
}

// truncate returns s unchanged when it fits within limit bytes, otherwise
// slices to limit bytes and appends a "...(truncated)" marker.
func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "...(truncated)"
}
