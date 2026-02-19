package agentfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Markers delimit the rimba-managed block within shared Markdown files.
	BeginMarker = "<!-- BEGIN RIMBA -->"
	EndMarker   = "<!-- END RIMBA -->"

	actionCreated = "created"
	actionUpdated = "updated"
	actionRemoved = "removed"
	actionSkipped = "skipped"
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

// Specs returns the specifications for all agent instruction files.
func Specs() []Spec {
	return []Spec{
		{RelPath: "AGENTS.md", Kind: KindBlock, Content: agentsBlock},
		{RelPath: filepath.Join(".github", "copilot-instructions.md"), Kind: KindBlock, Content: copilotBlock},
		{RelPath: filepath.Join(".cursor", "rules", "rimba.mdc"), Kind: KindWhole, Content: cursorContent},
		{RelPath: filepath.Join(".claude", "skills", "rimba", "SKILL.md"), Kind: KindWhole, Content: claudeSkillContent},
	}
}

// Install creates or updates all agent instruction files under repoRoot.
// It is idempotent: re-running replaces existing content.
func Install(repoRoot string) ([]Result, error) {
	var results []Result
	for _, spec := range Specs() {
		r, err := installOne(repoRoot, spec)
		if err != nil {
			return results, fmt.Errorf("%s: %w", spec.RelPath, err)
		}
		results = append(results, r)
	}
	return results, nil
}

// Uninstall removes rimba content from all agent instruction files under repoRoot.
func Uninstall(repoRoot string) ([]Result, error) {
	var results []Result
	for _, spec := range Specs() {
		r, err := uninstallOne(repoRoot, spec)
		if err != nil {
			return results, fmt.Errorf("%s: %w", spec.RelPath, err)
		}
		results = append(results, r)
	}
	return results, nil
}

// Status checks the installation state of all agent instruction files.
func Status(repoRoot string) []FileStatus {
	specs := Specs()
	statuses := make([]FileStatus, 0, len(specs))
	for _, spec := range specs {
		statuses = append(statuses, checkOne(repoRoot, spec))
	}
	return statuses
}

// ensureDir creates the directory (and parents) if possible.
// Returns false if creation fails (e.g. a parent path component is a regular file).
func ensureDir(dir string) bool {
	return os.MkdirAll(dir, 0750) == nil //nolint:gosec // dir needs to be accessible
}

func installOne(repoRoot string, spec Spec) (Result, error) {
	path := filepath.Join(repoRoot, spec.RelPath)

	if spec.Kind == KindWhole {
		return installWhole(path, spec)
	}
	return installBlock(path, spec)
}

func installWhole(path string, spec Spec) (Result, error) {
	if !ensureDir(filepath.Dir(path)) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	action := actionCreated
	if _, err := os.Stat(path); err == nil {
		action = actionUpdated
	}

	if err := os.WriteFile(path, []byte(spec.Content()), 0644); err != nil { //nolint:gosec // config file, not executable
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: action}, nil
}

func installBlock(path string, spec Spec) (Result, error) {
	if !ensureDir(filepath.Dir(path)) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("read file: %w", err)
	}

	content := string(existing)
	block := spec.Content()
	action := actionCreated

	if content != "" {
		if containsBlock(content) {
			// Replace existing block
			content = removeBlock(content)
			action = actionUpdated
		} else {
			action = actionUpdated
		}
		// Append block to existing content (with removed old block if any)
		content = strings.TrimRight(content, "\n")
		if content != "" {
			content = content + "\n\n" + block + "\n"
		} else {
			content = block + "\n"
		}
	} else {
		content = block + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil { //nolint:gosec // config file, not executable
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: action}, nil
}

func uninstallOne(repoRoot string, spec Spec) (Result, error) {
	path := filepath.Join(repoRoot, spec.RelPath)

	if spec.Kind == KindWhole {
		return uninstallWhole(path, spec)
	}
	return uninstallBlock(path, spec)
}

func uninstallWhole(path string, spec Spec) (Result, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	if err := os.Remove(path); err != nil {
		return Result{RelPath: spec.RelPath}, fmt.Errorf("remove file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
}

func uninstallBlock(path string, spec Spec) (Result, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
		}
		return Result{RelPath: spec.RelPath}, fmt.Errorf("read file: %w", err)
	}

	content := string(existing)
	if !containsBlock(content) {
		return Result{RelPath: spec.RelPath, Action: actionSkipped}, nil
	}

	cleaned := removeBlock(content)
	if strings.TrimSpace(cleaned) == "" {
		if err := os.Remove(path); err != nil {
			return Result{RelPath: spec.RelPath}, fmt.Errorf("remove file: %w", err)
		}
		return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
	}

	if err := os.WriteFile(path, []byte(cleaned), 0644); err != nil { //nolint:gosec // config file, not executable
		return Result{RelPath: spec.RelPath}, fmt.Errorf("write file: %w", err)
	}
	return Result{RelPath: spec.RelPath, Action: actionRemoved}, nil
}

func checkOne(repoRoot string, spec Spec) FileStatus {
	path := filepath.Join(repoRoot, spec.RelPath)

	if spec.Kind == KindWhole {
		_, err := os.Stat(path)
		return FileStatus{RelPath: spec.RelPath, Installed: err == nil}
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return FileStatus{RelPath: spec.RelPath, Installed: false}
	}
	return FileStatus{RelPath: spec.RelPath, Installed: containsBlock(string(existing))}
}

// containsBlock checks whether content includes the rimba marker block.
func containsBlock(content string) bool {
	return strings.Contains(content, BeginMarker) && strings.Contains(content, EndMarker)
}

// removeBlock strips the rimba marker block from content.
func removeBlock(content string) string {
	before, afterBegin, found := strings.Cut(content, BeginMarker)
	if !found {
		return content
	}

	_, afterEnd, found := strings.Cut(afterBegin, EndMarker)
	if !found {
		// Corrupt: BEGIN without END — remove from BEGIN to end of file
		before = strings.TrimRight(before, "\n")
		if before == "" {
			return ""
		}
		return before + "\n"
	}

	after := strings.TrimLeft(afterEnd, "\n")
	before = strings.TrimRight(before, "\n")

	if before == "" {
		return after
	}
	if after == "" {
		return before + "\n"
	}
	return before + "\n" + after
}
