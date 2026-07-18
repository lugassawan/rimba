// Package metrics records per-phase timing for a rimba command invocation and
// flushes it as a JSONL run record. It is a leaf package: stdlib-only
// dependencies, so internal/deps and internal/config can import it without
// creating an import cycle.
package metrics

import "runtime"

// SchemaVersion is the current Run schema version. Bump when the on-disk
// JSONL shape changes in a way consumers need to detect.
const SchemaVersion = 1

// DefaultMaxRuns is used when Flush's maxRuns argument is <= 0.
const DefaultMaxRuns = 500

// Span is one named, timed segment of a command invocation (e.g. "copy",
// "deps", a per-module deps install, or a per-hook run).
type Span struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"duration_ms"`
	Cloned     *bool  `json:"cloned,omitempty"` // deps module spans only
}

// MachineInfo captures host details at flush time, for cross-machine
// comparison of recorded runs.
type MachineInfo struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	NumCPU int    `json:"num_cpu"`
}

// Run is one JSONL record: the full span breakdown for a single command
// invocation.
type Run struct {
	SchemaVersion int         `json:"schema_version"`
	Timestamp     string      `json:"timestamp"` // RFC3339, UTC
	Command       string      `json:"command"`
	Task          string      `json:"task,omitempty"`
	Service       string      `json:"service,omitempty"`
	DurationMS    int64       `json:"duration_ms"`
	Machine       MachineInfo `json:"machine"`
	Spans         []Span      `json:"spans"`
}

// currentMachine captures machine info at flush time. A package var so tests
// can stub it for deterministic assertions.
var currentMachine = func() MachineInfo {
	return MachineInfo{OS: runtime.GOOS, Arch: runtime.GOARCH, NumCPU: runtime.NumCPU()}
}
