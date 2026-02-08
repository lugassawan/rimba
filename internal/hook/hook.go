package hook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	BeginMarker = "# BEGIN RIMBA HOOK"
	EndMarker   = "# END RIMBA HOOK"
	HookName    = "post-merge"
	shebang     = "#!/bin/sh"
	fileMode    = 0755
)

var (
	ErrAlreadyInstalled = errors.New("rimba hook is already installed")
	ErrNotInstalled     = errors.New("rimba hook is not installed")
)

// Status describes the current state of the post-merge hook.
type Status struct {
	Installed bool
	HookPath  string
	HasOther  bool // true if hook file has non-rimba content
}

// HookBlock returns the marker-delimited block with the branch guard embedded.
func HookBlock(branch string) string {
	return fmt.Sprintf(`%s
# Installed by rimba — do not edit this block manually
if command -v rimba >/dev/null 2>&1; then
  _rimba_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
  if [ "$_rimba_branch" = "%s" ]; then
    rimba clean --merged --force 2>/dev/null || true
  fi
fi
%s`, BeginMarker, branch, EndMarker)
}

// Install creates or appends the rimba hook block to the post-merge hook file.
func Install(hooksDir, branch string) error {
	if err := os.MkdirAll(hooksDir, 0750); err != nil { //nolint:gosec // hooks dir needs exec bit for git
		return fmt.Errorf("create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, HookName)
	existing, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read hook file: %w", err)
	}

	content := string(existing)
	if containsBlock(content) {
		return ErrAlreadyInstalled
	}

	block := HookBlock(branch)
	var newContent string
	if content == "" {
		newContent = shebang + "\n\n" + block + "\n"
	} else {
		newContent = content + "\n" + block + "\n"
	}

	if err := os.WriteFile(hookPath, []byte(newContent), fileMode); err != nil {
		return fmt.Errorf("write hook file: %w", err)
	}

	return nil
}

// Uninstall removes the rimba hook block from the post-merge hook file.
func Uninstall(hooksDir string) error {
	hookPath := filepath.Join(hooksDir, HookName)
	existing, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotInstalled
		}
		return fmt.Errorf("read hook file: %w", err)
	}

	content := string(existing)
	if !containsBlock(content) {
		return ErrNotInstalled
	}

	cleaned := removeBlock(content)
	if isShebangOnly(cleaned) {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("remove hook file: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(hookPath, []byte(cleaned), fileMode); err != nil {
		return fmt.Errorf("write hook file: %w", err)
	}
	return nil
}

// Check inspects the current state of the post-merge hook.
func Check(hooksDir string) Status {
	hookPath := filepath.Join(hooksDir, HookName)
	existing, err := os.ReadFile(hookPath)
	if err != nil {
		return Status{HookPath: hookPath}
	}

	content := string(existing)
	installed := containsBlock(content)

	hasOther := false
	if installed {
		cleaned := removeBlock(content)
		hasOther = !isShebangOnly(cleaned)
	} else {
		hasOther = !isShebangOnly(content)
	}

	return Status{
		Installed: installed,
		HookPath:  hookPath,
		HasOther:  hasOther,
	}
}

func containsBlock(content string) bool {
	return strings.Contains(content, BeginMarker) && strings.Contains(content, EndMarker)
}

func removeBlock(content string) string {
	beginIdx := strings.Index(content, BeginMarker)
	if beginIdx == -1 {
		return content
	}

	endIdx := strings.Index(content, EndMarker)
	if endIdx == -1 {
		// Corrupt: BEGIN without END — remove from BEGIN to end of file
		before := strings.TrimRight(content[:beginIdx], "\n")
		if before == "" {
			return ""
		}
		return before + "\n"
	}

	// Remove from BEGIN marker through END marker (including trailing newline)
	after := content[endIdx+len(EndMarker):]
	after = strings.TrimLeft(after, "\n")

	before := content[:beginIdx]
	before = strings.TrimRight(before, "\n")

	if before == "" {
		return after
	}
	if after == "" {
		return before + "\n"
	}
	return before + "\n" + after
}

func isShebangOnly(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed == "" || trimmed == shebang
}
