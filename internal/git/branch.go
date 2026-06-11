package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/errhint"
)

const internalGitInvariantHint = "report this — git output unexpectedly malformed"

// errEmptyDiff is returned by ComputePatchIDs when the input diff is empty.
var errEmptyDiff = errors.New("empty diff")

// ComputePatchIDs computes patch-ids from a diff string by piping it to
// git patch-id --stable. Defined as a variable so tests can override it
// with deterministic results without executing a real git subprocess.
// Returns the set of patch-id hex strings (first field per output line).
//
// Not safe for concurrent modification — tests must override and restore
// sequentially via defer:
//
//	defer func(orig ...) { ComputePatchIDs = orig }(ComputePatchIDs)
//	ComputePatchIDs = func(...) { ... }
var ComputePatchIDs = func(ctx context.Context, diff string) (map[string]bool, error) {
	if diff == "" {
		return nil, errEmptyDiff
	}
	cmd := exec.CommandContext(ctx, "git", "patch-id", "--stable")
	cmd.Env = stableGitEnv(os.Environ())
	cmd.Stdin = strings.NewReader(diff)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git patch-id --stable: %w", err)
	}

	ids := make(map[string]bool)
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, _, _ := strings.Cut(line, " ")
		if pid != "" {
			ids[pid] = true
		}
	}
	return ids, nil
}

// BranchExists checks whether a local branch exists.
func BranchExists(ctx context.Context, r Runner, branch string) bool {
	_, err := r.Run(ctx, cmdRevParse, flagVerify, refsHeadsPrefix+branch)
	return err == nil
}

// DeleteBranch deletes a local branch. If force is true, uses -D instead of -d.
// Already-gone branches are treated as success (idempotent).
func DeleteBranch(ctx context.Context, r Runner, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.Run(ctx, "branch", flag, branch)
	// git emits "error: branch 'X' not found." — assumes LC_ALL=C or English git.
	if err != nil && strings.Contains(err.Error(), "branch '") && strings.Contains(err.Error(), "not found") {
		return nil // already gone — idempotent
	}
	return err
}

// RenameBranch renames a local branch from oldBranch to newBranch.
func RenameBranch(ctx context.Context, r Runner, oldBranch, newBranch string) error {
	_, err := r.Run(ctx, "branch", "-m", oldBranch, newBranch)
	return err
}

// CurrentBranch returns the short branch name checked out in the given directory.
// Returns an error with a hint if HEAD is detached.
func CurrentBranch(ctx context.Context, r Runner, dir string) (string, error) {
	out, err := r.RunInDir(ctx, dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", errhint.WithFix(
			fmt.Errorf("could not determine current branch: %w", err),
			"checkout a branch first: git checkout <branch>",
		)
	}
	return strings.TrimSpace(out), nil
}

// Checkout switches the working tree in dir to the given branch.
func Checkout(ctx context.Context, r Runner, dir, branch string) error {
	_, err := r.RunInDir(ctx, dir, "switch", "--", branch)
	return err
}

// IsDirty returns true if the working tree at the given directory has uncommitted changes.
func IsDirty(ctx context.Context, r Runner, dir string) (bool, error) {
	out, err := r.RunInDir(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// AheadBehind returns the ahead/behind counts of the current branch vs its upstream.
// Returns (0, 0, nil) if there's no upstream configured.
// Returns ctx.Err() on context cancellation so callers can distinguish a
// timed-out query from a branch with no upstream.
func AheadBehind(ctx context.Context, r Runner, dir string) (ahead, behind int, _ error) {
	out, err := r.RunInDir(ctx, dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		// exec.CommandContext kills via SIGKILL; CombinedOutput returns *exec.ExitError,
		// not the context sentinel — check ctx.Err() directly.
		if ctx.Err() != nil {
			return 0, 0, ctx.Err()
		}
		// No upstream configured or other non-fatal error — treat as 0/0.
		return 0, 0, nil //nolint:nilerr // intentional: missing upstream is not an error
	}

	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, nil
	}

	// parts[0] = upstream count (behind), parts[1] = HEAD count (ahead)
	parseCount(parts[0], &behind)
	parseCount(parts[1], &ahead)
	return ahead, behind, nil
}

// IsSquashMerged checks whether a branch's content has been squash-merged into mergeRef.
// It computes a patch-id for the branch diff and checks whether any commit in
// mergeRef (since the merge-base) produces the same patch-id.
// Unlike the previous commit-tree + cherry approach, this technique creates no
// loose objects in the git store.
func IsSquashMerged(ctx context.Context, r Runner, mergeRef, branch string) (bool, error) {
	mergeBase, err := MergeBase(ctx, r, mergeRef, branch)
	if err != nil {
		return false, err
	}

	tip, err := r.Run(ctx, cmdRevParse, "--verify", branch)
	if err != nil {
		return false, err
	}

	// Empty branch (merge-base == branch tip) — nothing to squash-merge.
	if mergeBase == tip {
		return false, nil
	}

	// Branch patch-id: diff from merge-base to branch tip.
	branchDiff, err := r.Run(ctx, "diff", mergeBase, branch)
	if err != nil {
		return false, err
	}
	branchPIDs, err := ComputePatchIDs(ctx, branchDiff)
	if err != nil {
		if errors.Is(err, errEmptyDiff) {
			return false, nil // empty diff — not squash-merged
		}
		return false, err
	}
	if len(branchPIDs) == 0 {
		return false, nil
	}

	// MergeRef patch-ids: diffs for each non-merge commit since merge-base.
	mergeRefDiffs, err := r.Run(ctx, "log", "-p", "--no-merges", mergeBase+".."+mergeRef)
	if err != nil {
		return false, err
	}
	mergeRefPIDs, err := ComputePatchIDs(ctx, mergeRefDiffs)
	if err != nil {
		if errors.Is(err, errEmptyDiff) {
			return false, nil // no commits in mergeRef range
		}
		return false, err
	}

	for pid := range mergeRefPIDs {
		if branchPIDs[pid] {
			return true, nil
		}
	}
	return false, nil
}

// MergedBranches returns branches that have been merged into the given branch.
// Runs `git branch --merged <branch>` and parses the output.
func MergedBranches(ctx context.Context, r Runner, branch string) ([]string, error) {
	out, err := r.Run(ctx, "branch", "--merged", branch)
	if err != nil {
		return nil, err
	}

	var branches []string
	for line := range strings.SplitSeq(out, "\n") {
		// Lines are "  branch-name", "* current-branch", or "+ worktree-branch"
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "+ ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// LastCommitTime returns the time of the last commit on the given branch.
func LastCommitTime(ctx context.Context, r Runner, branch string) (time.Time, error) {
	t, _, err := LastCommitInfo(ctx, r, branch)
	return t, err
}

// LastCommitInfo returns the time and subject of the last commit on the given branch.
func LastCommitInfo(ctx context.Context, r Runner, branch string) (time.Time, string, error) {
	out, err := r.Run(ctx, "log", "-1", "--format=%ct\t%s", branch)
	if err != nil {
		return time.Time{}, "", errhint.WithFix(
			fmt.Errorf("last commit info for %s: %w", branch, err),
			"verify the branch exists: git branch --list <branch>",
		)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return time.Time{}, "", errhint.WithFix(
			fmt.Errorf("no commits on branch %s", branch),
			"add a commit on the branch before running this",
		)
	}

	tsStr, subject, found := strings.Cut(out, "\t")
	if !found {
		return time.Time{}, "", errhint.WithFix(
			fmt.Errorf("malformed commit info %q", out),
			internalGitInvariantHint,
		)
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return time.Time{}, "", errhint.WithFix(
			fmt.Errorf("parse commit timestamp %q: %w", tsStr, err),
			internalGitInvariantHint,
		)
	}
	return time.Unix(ts, 0), subject, nil
}

// LocalBranches returns the list of local branch names.
func LocalBranches(ctx context.Context, r Runner) ([]string, error) {
	out, err := r.Run(ctx, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var branches []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func parseCount(s string, v *int) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	*v = n
}
