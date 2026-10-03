package operations

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

// RollbackParams describes a just-created worktree whose post-create setup failed.
type RollbackParams struct {
	WtPath     string
	Branch     string
	Task       string
	KeepBranch bool // true when the branch pre-existed the create (restore); it is preserved
	OnProgress progress.Func
}

// RollbackFailedCreate compensates a failed create+setup pair by removing the
// just-added worktree and, unless it pre-existed the create, its branch.
// Cleanup is intentionally non-cancellable; setupErr stays errors.Is-reachable
// while its stale manual-recovery hints are suppressed from the rendered message.
func RollbackFailedCreate(r git.Runner, p RollbackParams, setupErr error) error {
	wrapped := hintlessError{err: setupErr}
	result, err := RemoveWorktree(
		context.Background(),
		r,
		resolver.WorktreeInfo{Path: p.WtPath, Branch: p.Branch},
		p.Task,
		p.KeepBranch,
		true,
		p.OnProgress,
	)
	dirRemains := func() bool {
		_, statErr := os.Stat(p.WtPath)
		return statErr == nil
	}
	switch {
	case err != nil && (result.LeftOnDisk || dirRemains()):
		return fmt.Errorf("%w\nRollback failed for worktree %s; branch preserved: %s\nDirectory remains; remove it manually: %s\nCleanup error: %w", wrapped, p.WtPath, p.Branch, p.WtPath, err)
	case err != nil:
		return fmt.Errorf("%w\nRollback failed for worktree %s; branch preserved: %s\nCleanup error: %w", wrapped, p.WtPath, p.Branch, err)
	case result.BranchError != nil && result.LeftOnDisk:
		return fmt.Errorf("%w\nRollback incomplete: branch %s remains and directory remains: %s\nRemove the directory manually: %s\nBranch cleanup error: %w", wrapped, p.Branch, p.WtPath, p.WtPath, result.BranchError)
	case result.BranchError != nil:
		return fmt.Errorf("%w\nRollback incomplete: worktree removed but branch %s remains: %w", wrapped, p.Branch, result.BranchError)
	case result.LeftOnDisk && p.KeepBranch:
		return fmt.Errorf("%w\nRollback incomplete: worktree removed, but directory remains: %s\nRemove the directory manually: %s", wrapped, p.WtPath, p.WtPath)
	case result.LeftOnDisk:
		return fmt.Errorf("%w\nRollback incomplete: branch %s was removed, but directory remains: %s\nRemove the directory manually: %s", wrapped, p.Branch, p.WtPath, p.WtPath)
	case p.KeepBranch:
		return fmt.Errorf("%w\nRollback completed: removed worktree %s; branch %s preserved; retry the command", wrapped, p.WtPath, p.Branch)
	default:
		return fmt.Errorf("%w\nRollback completed: removed worktree %s and branch %s; retry the command", wrapped, p.WtPath, p.Branch)
	}
}

// hintlessError renders err without stale manual-recovery hint lines so a
// completed rollback owns the recovery guidance, keeping the chain for errors.Is.
type hintlessError struct{ err error }

func (e hintlessError) Error() string { return stripRecoveryHints(e.err.Error()) }
func (e hintlessError) Unwrap() error { return e.err }

func stripRecoveryHints(msg string) string {
	lines := strings.Split(msg, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "To fix:") || strings.HasPrefix(t, "To retry,") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
