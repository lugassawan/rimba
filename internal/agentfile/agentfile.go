// Package agentfile manages agent instruction files for rimba.
package agentfile

const (
	// Markers delimit the rimba-managed block within shared Markdown files.
	BeginMarker = "<!-- BEGIN RIMBA -->"
	EndMarker   = "<!-- END RIMBA -->"

	actionCreated         = "created"
	actionUpdated         = "updated"
	actionRemoved         = "removed"
	actionSkipped         = "skipped"
	actionRegistered      = "registered"
	actionUnregistered    = "unregistered"
	actionUnchanged       = "unchanged"
	actionSkippedNoConfig = "skipped — no config"
)

// FileKind distinguishes installation strategies.
type FileKind int

const (
	// KindBlock injects a marker-delimited block into a shared file.
	KindBlock FileKind = iota
	// KindWhole creates/overwrites an entire file owned by rimba.
	KindWhole
)

// Spec describes a single agent instruction file.
type Spec struct {
	RelPath string        // e.g. "AGENTS.md", ".cursor/rules/rimba.mdc"
	Kind    FileKind      // block-based or whole-file
	Content func() string // returns the content to write
}

// Result reports what happened to a single file during Install or Uninstall.
type Result struct {
	RelPath string
	Action  string // "created", "updated", "removed", "skipped"
}

// FileStatus reports the installation state of a single agent file.
type FileStatus struct {
	RelPath   string
	Installed bool
}
