package hook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Markers delimit the rimba-managed block within a hook file.
	// Each hook type lives in its own file, so shared markers do not collide.
	BeginMarker   = "# BEGIN RIMBA HOOK"
	EndMarker     = "# END RIMBA HOOK"
	PostMergeHook = "post-merge"
	PreCommitHook = "pre-commit"
	shebang       = "#!/bin/sh"
	fileMode      = 0755
)

var (
	ErrAlreadyInstalled = errors.New("rimba hook is already installed")
	ErrNotInstalled     = errors.New("rimba hook is not installed")
	ErrCorruptBlock     = errors.New("corrupt rimba hook block; resolve manually")
)

// Status describes the current state of a hook.
type Status struct {
	Installed bool
	HookPath  string
	HasOther  bool // true if hook file has non-rimba content
	Corrupt   bool // true if file has a BEGIN marker without a matching END
}

// PostMergeBlock returns the marker-delimited block with the branch guard embedded.
func PostMergeBlock(branch string) string {
	//nolint:dupword // shell script has two "fi" closings
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

// Install creates or appends the rimba hook block to the given hook file.
func Install(hooksDir, hookName, block string) error {
	if err := os.MkdirAll(hooksDir, 0750); err != nil { //nolint:gosec // hooks dir needs exec bit for git
		return fmt.Errorf("create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, hookName)
	existing, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read hook file: %w", err)
	}

	content := string(existing)
	if isCorruptBlock(content) {
		return ErrCorruptBlock
	}
	if containsBlock(content) {
		return ErrAlreadyInstalled
	}

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

// Uninstall removes the rimba hook block from the given hook file.
func Uninstall(hooksDir, hookName string) error {
	hookPath := filepath.Join(hooksDir, hookName)
	existing, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotInstalled
		}
		return fmt.Errorf("read hook file: %w", err)
	}

	content := string(existing)
	if isCorruptBlock(content) {
		return ErrCorruptBlock
	}
	if !containsBlock(content) {
		return ErrNotInstalled
	}

	cleaned, err := removeBlock(content)
	if err != nil {
		return err
	}
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

// Check inspects the current state of the given hook.
func Check(hooksDir, hookName string) Status {
	hookPath := filepath.Join(hooksDir, hookName)
	existing, err := os.ReadFile(hookPath)
	if err != nil {
		return Status{HookPath: hookPath}
	}

	content := string(existing)
	if isCorruptBlock(content) {
		return Status{HookPath: hookPath, Corrupt: true}
	}

	installed := containsBlock(content)

	var hasOther bool
	if installed {
		// isCorruptBlock already ruled out an orphaned BEGIN, so removeBlock cannot error here.
		cleaned, _ := removeBlock(content)
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

// PreCommitBlock returns the marker-delimited block that prevents direct commits
// to main/master. Both branch names are hardcoded because the protection should
// apply regardless of which default branch the repository uses.
func PreCommitBlock() string {
	return fmt.Sprintf(`%s
# Installed by rimba — do not edit this block manually
_rimba_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)
if [ "$_rimba_branch" = "main" ] || [ "$_rimba_branch" = "master" ]; then
  echo "rimba: direct commits to $_rimba_branch are not allowed."
  echo "       Use 'rimba add <task>' to create a worktree branch."
  exit 1
fi
%s`, BeginMarker, EndMarker)
}

func containsBlock(content string) bool {
	return strings.Contains(content, BeginMarker) && strings.Contains(content, EndMarker)
}

func removeBlock(content string) (string, error) {
	before, afterBegin, found := strings.Cut(content, BeginMarker)
	if !found {
		return content, nil
	}

	_, afterEnd, found := strings.Cut(afterBegin, EndMarker)
	if !found {
		return "", ErrCorruptBlock
	}

	// Remove from BEGIN marker through END marker (including trailing newline)
	after := strings.TrimLeft(afterEnd, "\n")
	before = strings.TrimRight(before, "\n")

	if before == "" {
		return after, nil
	}
	if after == "" {
		return before + "\n", nil
	}
	return before + "\n" + after, nil
}

func isShebangOnly(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed == "" || trimmed == shebang
}

func isCorruptBlock(content string) bool {
	if strings.Count(content, BeginMarker) > 1 {
		return true
	}
	_, err := removeBlock(content)
	return errors.Is(err, ErrCorruptBlock)
}
