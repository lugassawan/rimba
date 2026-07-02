package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/errhint"
)

const internalGitInvariantHint = "report this — git output unexpectedly malformed"

// ComputePatchIDs pipes diff into git patch-id --stable and returns the set of
// patch-id hex strings. Exposed as a variable so tests can stub it without a
// real git subprocess. Not safe for concurrent modification.
var ComputePatchIDs = func(ctx context.Context, diff string) (map[string]bool, error) {
	if diff == "" {
		return map[string]bool{}, nil
	}
	cmd := exec.CommandContext(ctx, "git", "patch-id", "--stable")
	cmd.Env = stableGitEnv(os.Environ())
	cmd.Stdin = strings.NewReader(diff)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("git patch-id --stable: %s: %w", msg, err)
		}
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

// FirstParentChainSHAs returns the set of commit SHAs on mergeRef's mainline
// (first-parent) history.
func FirstParentChainSHAs(ctx context.Context, r Runner, mergeRef string) (map[string]bool, error) {
	out, err := r.Run(ctx, cmdRevList, flagFirstParent, mergeRef)
	if err != nil {
		return nil, err
	}
	shas := make(map[string]bool)
	for line := range strings.SplitSeq(out, "\n") {
		if sha := strings.TrimSpace(line); sha != "" {
			shas[sha] = true
		}
	}
	return shas, nil
}

// IsSHAOnChain reports whether sha is in chain (e.g. from FirstParentChainSHAs).
func IsSHAOnChain(sha string, chain map[string]bool) bool {
	return chain[sha]
}

// IsSquashMerged reports whether branch's diff patch-id matches any commit in
// mergeRef since their common merge-base, indicating a squash merge.
func IsSquashMerged(ctx context.Context, r Runner, mergeRef, branch string) (bool, error) {
	mergeBase, err := MergeBase(ctx, r, mergeRef, branch)
	if err != nil {
		return false, err
	}

	tip, err := r.Run(ctx, cmdRevParse, "--verify", branch)
	if err != nil {
		return false, err
	}

	if strings.TrimSpace(mergeBase) == strings.TrimSpace(tip) { // empty branch — nothing squash-merged
		return false, nil
	}

	branchDiff, err := r.Run(ctx, CmdDiff, mergeBase, branch)
	if err != nil {
		return false, err
	}
	branchPIDs, err := ComputePatchIDs(ctx, branchDiff)
	if err != nil {
		return false, err
	}
	if len(branchPIDs) == 0 {
		return false, nil
	}

	mergeRefDiffs, err := r.Run(ctx, CmdLog, "-p", "--no-merges", mergeBase+".."+mergeRef)
	if err != nil {
		return false, err
	}
	mergeRefPIDs, err := ComputePatchIDs(ctx, mergeRefDiffs)
	if err != nil {
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
	out, err := r.Run(ctx, CmdLog, "-1", "--format=%ct\t%s", branch)
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
