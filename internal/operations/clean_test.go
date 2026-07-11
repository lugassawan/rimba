package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
)

const (
	gitCmdBranch   = "branch"
	gitCmdLog      = "log"
	gitCmdWorktree = "worktree"
	gitSubcmdAdd   = "add"
	gitCmdPush     = "push"
)

// porcelainEntries builds porcelain-format output for git worktree list.
func porcelainEntries(entries ...struct{ path, branch string }) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString("worktree " + e.path + "\n")
		sb.WriteString("HEAD abc123\n")
		sb.WriteString("branch refs/heads/" + e.branch + "\n")
		sb.WriteString("\n")
	}
	return sb.String()
}

func TestFindMergedCandidatesNormalMerge(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/done", "feature/done"},
		struct{ path, branch string }{"/wt/active", "feature/active"},
		struct{ path, branch string }{"/wt/fixed", "bugfix/fixed"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "  feature/done\n  bugfix/fixed\n", nil
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			// IsSquashMerged fallback (feature/active): rev-parse tip for merge-base comparison.
			if len(args) > 0 && args[0] == cmdRevParse {
				return "tip456", nil
			}
			// classifyMergedEntry: merge-commit merges leave the tip off mainline.
			if len(args) > 0 && args[0] == gitCmdRevList {
				return "otherSha1\notherSha2", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(result.Candidates))
	}
	if result.Candidates[0].Branch != "feature/done" {
		t.Errorf("expected feature/done, got %s", result.Candidates[0].Branch)
	}
	if result.Candidates[1].Branch != "bugfix/fixed" {
		t.Errorf("expected bugfix/fixed, got %s", result.Candidates[1].Branch)
	}
}

// TestFindMergedCandidatesFreshWorktreeNotRemoved guards issue #335: a fresh
// worktree's branch appears in `git branch --merged` but must not be removed.
func TestFindMergedCandidatesFreshWorktreeNotRemoved(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/fresh", "feature/fresh"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// feature/fresh is reported as "merged" because its tip is the base commit.
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "  feature/fresh\n", nil
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			// classifyMergedEntry checks e.HEAD (from the porcelain "HEAD abc123"
			// line) against the mainline set — it's on it, so the entry is protected.
			if len(args) > 0 && args[0] == gitCmdRevList {
				return "abc123\nolder", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("expected 0 candidates (fresh worktree must be protected), got %d: %+v", len(result.Candidates), result.Candidates)
	}
}

// TestFindMergedCandidatesPrunableEntry guards #374: a worktree whose .git
// file was deleted out-of-band still shows up in `git branch --merged`
// (its ref still exists) but git also marks the porcelain entry prunable.
// The resulting candidate must carry Prunable so removal routes through
// git worktree prune instead of the doomed git worktree remove --force.
func TestFindMergedCandidatesPrunableEntry(t *testing.T) {
	wt := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /wt/broken",
		"HEAD def456",
		"branch refs/heads/feature/broken",
		"prunable gitdir file points to non-existent location",
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "  feature/broken\n", nil
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) > 0 && args[0] == gitCmdRevList {
				return "otherSha1\notherSha2", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %+v", len(result.Candidates), result.Candidates)
	}
	if !result.Candidates[0].Prunable {
		t.Error("expected candidate.Prunable = true for a worktree with a deleted .git file")
	}
}

// TestFindMergedCandidatesMergeCommitRemoved locks in the merge-commit-merged
// case: the branch tip is off mainline, so it must still be removed.
func TestFindMergedCandidatesMergeCommitRemoved(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/merged", "feature/merged"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "  feature/merged\n", nil
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			// classifyMergedEntry checks e.HEAD ("abc123") against the mainline
			// set — it's absent, so the entry is off mainline and removable.
			if len(args) > 0 && args[0] == gitCmdRevList {
				return "mainlineSha1\nmainlineSha2", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate (merge-commit-merged branch must be removed), got %d", len(result.Candidates))
	}
	if result.Candidates[0].Branch != "feature/merged" {
		t.Errorf("expected feature/merged, got %s", result.Candidates[0].Branch)
	}
}

func TestFindMergedCandidatesSquashMerge(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		git.ComputePatchIDs = orig
	}(git.ComputePatchIDs)
	git.ComputePatchIDs = func(_ context.Context, _ string) (map[string]bool, error) {
		return map[string]bool{"fakePID": true}, nil
	}

	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/squashed", "feature/squashed"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "", nil // No normal merges
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			// IsSquashMerged: merge-base → rev-parse → diff → log
			if len(args) > 0 && args[0] == git.CmdMergeBase {
				return "base123", nil
			}
			if len(args) > 0 && args[0] == cmdRevParse {
				return "tip456", nil
			}
			if len(args) > 0 && args[0] == git.CmdDiff {
				return "fake diff", nil
			}
			if len(args) > 0 && args[0] == git.CmdLog {
				return "fake log", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
}

// sharedMergeBaseRunFunc backs TestFindMergedCandidatesSquashMergeCachesMainlinePatchIDsByMergeBase:
// two candidates that share a merge-base, counting mainline log invocations.
func sharedMergeBaseRunFunc(wt string, logInvocations *int) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == gitCmdBranch:
			return "", nil // no normal merges — force squash-check path for both
		case len(args) > 0 && args[0] == gitCmdWorktree:
			return wt, nil
		case len(args) > 0 && args[0] == git.CmdMergeBase:
			return "shared-base", nil // both branches share the same merge-base
		case len(args) > 0 && args[0] == cmdRevParse:
			return "tip-" + args[len(args)-1], nil // distinct tip, never equals mergeBase
		case len(args) > 0 && args[0] == git.CmdDiff:
			return "diff-" + args[len(args)-1], nil
		case len(args) > 0 && args[0] == git.CmdLog:
			*logInvocations++
			return "mainline-log", nil
		}
		return "", nil
	}
}

// TestFindMergedCandidatesSquashMergeCachesMainlinePatchIDsByMergeBase locks in
// finding #5: the mainline patch-ID computation runs once per merge-base, not per branch.
func TestFindMergedCandidatesSquashMergeCachesMainlinePatchIDsByMergeBase(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		git.ComputePatchIDs = orig
	}(git.ComputePatchIDs)

	git.ComputePatchIDs = func(_ context.Context, diff string) (map[string]bool, error) {
		if diff == "mainline-log" {
			return map[string]bool{"mainlinePID": true}, nil
		}
		return map[string]bool{"branchPID": true}, nil // never matches mainlinePID
	}

	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/a", "feature/a"},
		struct{ path, branch string }{"/wt/b", "feature/b"},
	)

	logInvocations := 0
	r := &mockRunner{
		run:      sharedMergeBaseRunFunc(wt, &logInvocations),
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("expected 0 candidates (branchPID never matches mainlinePID), got %d: %+v", len(result.Candidates), result.Candidates)
	}
	if logInvocations != 1 {
		t.Errorf("expected mainline log to be computed once for the shared merge-base, got %d invocations", logInvocations)
	}
}

// distinctMergeBasePatchIDs backs TestFindMergedCandidatesSquashMergeDistinctMergeBasesNotConflated.
func distinctMergeBasePatchIDs(_ context.Context, diff string) (map[string]bool, error) {
	switch diff {
	case "mainline-log-a":
		return map[string]bool{"sharedPID": true}, nil
	case "mainline-log-b":
		return map[string]bool{"otherPID": true}, nil
	case "diff-feature/a":
		return map[string]bool{"sharedPID": true}, nil // matches base-a's mainline
	default:
		return map[string]bool{"branchOnlyPID": true}, nil // matches neither
	}
}

// distinctMergeBaseRunFunc backs TestFindMergedCandidatesSquashMergeDistinctMergeBasesNotConflated:
// two candidates with different merge-bases, counting log calls per range.
func distinctMergeBaseRunFunc(wt string, logCallsByRange map[string]int) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		switch {
		case len(args) > 0 && args[0] == gitCmdBranch:
			return "", nil
		case len(args) > 0 && args[0] == gitCmdWorktree:
			return wt, nil
		case len(args) > 0 && args[0] == git.CmdMergeBase:
			if args[len(args)-1] == "feature/a" {
				return "base-a", nil
			}
			return "base-b", nil
		case len(args) > 0 && args[0] == cmdRevParse:
			return "tip-" + args[len(args)-1], nil
		case len(args) > 0 && args[0] == git.CmdDiff:
			return "diff-" + args[len(args)-1], nil
		case len(args) > 0 && args[0] == git.CmdLog:
			rangeArg := args[len(args)-1]
			logCallsByRange[rangeArg]++
			if rangeArg == "base-a..origin/main" {
				return "mainline-log-a", nil
			}
			return "mainline-log-b", nil
		}
		return "", nil
	}
}

// TestFindMergedCandidatesSquashMergeDistinctMergeBasesNotConflated ensures distinct
// merge-bases get their own computation, and a match for one doesn't leak into the other.
func TestFindMergedCandidatesSquashMergeDistinctMergeBasesNotConflated(t *testing.T) {
	defer func(orig func(context.Context, string) (map[string]bool, error)) {
		git.ComputePatchIDs = orig
	}(git.ComputePatchIDs)

	git.ComputePatchIDs = distinctMergeBasePatchIDs

	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/a", "feature/a"},
		struct{ path, branch string }{"/wt/b", "feature/b"},
	)

	logCallsByRange := map[string]int{}
	r := &mockRunner{
		run:      distinctMergeBaseRunFunc(wt, logCallsByRange),
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Branch != "feature/a" {
		t.Fatalf("expected only feature/a as a candidate, got %+v", result.Candidates)
	}
	if logCallsByRange["base-a..origin/main"] != 1 || logCallsByRange["base-b..origin/main"] != 1 {
		t.Errorf("expected one mainline log call per distinct merge-base, got %v", logCallsByRange)
	}
}

func TestFindMergedCandidatesNoCandidates(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "", nil
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}
}

func TestFindMergedCandidatesGitError(t *testing.T) {
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", errors.New("git failed") },
		runInDir: noopRunInDir,
	}

	_, err := FindMergedCandidates(context.Background(), r, "origin/main", "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindStaleCandidatesFound(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/old", "feature/old"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) > 0 && args[0] == gitCmdLog {
				// Unix epoch for 2020-01-01 with a tab-separated subject
				return "1577836800\told commit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindStaleCandidates(context.Background(), r, "main", 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	if result.Candidates[0].Branch != "feature/old" {
		t.Errorf("expected feature/old, got %s", result.Candidates[0].Branch)
	}
}

func TestFindStaleCandidatesPrunableEntry(t *testing.T) {
	wt := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /wt/broken",
		"HEAD def456",
		"branch refs/heads/feature/broken",
		"prunable gitdir file points to non-existent location",
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) > 0 && args[0] == gitCmdLog {
				return "1577836800\told commit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindStaleCandidates(context.Background(), r, "main", 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
	if !result.Candidates[0].Prunable {
		t.Error("expected candidate.Prunable = true for a worktree with a deleted .git file")
	}
}

func TestFindStaleCandidatesNoneStale(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/fresh", "feature/fresh"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) > 0 && args[0] == gitCmdLog {
				// Unix epoch for 2099-01-01 with a tab-separated subject
				return "4070908800\tfresh commit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindStaleCandidates(context.Background(), r, "main", 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}
}

func TestFindStaleCandidatesGitError(t *testing.T) {
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", errors.New("git failed") },
		runInDir: noopRunInDir,
	}

	_, err := FindStaleCandidates(context.Background(), r, "main", 14)
	if err == nil {
		t.Fatal("expected error")
	}
}

func assertCleanedItem(t *testing.T, item CleanedItem, wantRemoved, wantDeleted, wantErr bool) {
	t.Helper()
	if item.WorktreeRemoved != wantRemoved {
		t.Errorf("WorktreeRemoved = %v, want %v", item.WorktreeRemoved, wantRemoved)
	}
	if item.BranchDeleted != wantDeleted {
		t.Errorf("BranchDeleted = %v, want %v", item.BranchDeleted, wantDeleted)
	}
	if wantErr && item.Error == nil {
		t.Error("expected error, got nil")
	}
	if !wantErr && item.Error != nil {
		t.Errorf("expected no error, got %v", item.Error)
	}
}

func TestRemoveCandidatesMixedResults(t *testing.T) {
	// .git present == genuine failure, not an orphan the heal path would retry.
	failingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(failingDir, ".git"), []byte("gitdir: /somewhere/.git/worktrees/b\n"), 0o644); err != nil {
		t.Fatalf("failed to create .git fixture: %v", err)
	}

	callCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdWorktree {
				callCount++
				if callCount == 2 {
					return "", errors.New("removal failed")
				}
				return "", nil
			}
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates := []CleanCandidate{
		{Path: "/wt/a", Branch: "feature/a"},
		{Path: failingDir, Branch: "feature/b"}, // Will fail removal
		{Path: "/wt/c", Branch: "feature/c"},
	}

	items := RemoveCandidates(context.Background(), r, candidates, false, false, nil)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	t.Run("success", func(t *testing.T) {
		assertCleanedItem(t, items[0], true, true, false)
	})
	t.Run("failed removal", func(t *testing.T) {
		assertCleanedItem(t, items[1], false, false, true)
	})
	t.Run("success after failure", func(t *testing.T) {
		assertCleanedItem(t, items[2], true, true, false)
	})
}

func TestRemoveCandidatesPrunableInputHealsAndRemoves(t *testing.T) {
	var repairInvoked, removeInvoked, pruneInvoked bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree repair"):
				repairInvoked = true
			case strings.Contains(cmd, "worktree remove"):
				removeInvoked = true
			case strings.Contains(cmd, "worktree prune"):
				pruneInvoked = true
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates := []CleanCandidate{
		{Path: "/wt/broken", Branch: "feature/broken", Prunable: true},
	}

	items := RemoveCandidates(context.Background(), r, candidates, false, false, nil)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].WorktreeRemoved {
		t.Error("expected WorktreeRemoved = true")
	}
	if items[0].Prunable {
		t.Error("expected Prunable = false — repair+remove fully cleared the directory")
	}
	if !repairInvoked {
		t.Error("expected 'git worktree repair' to be invoked for a prunable candidate")
	}
	if !removeInvoked {
		t.Error("expected 'git worktree remove' to be invoked after repair")
	}
	if pruneInvoked {
		t.Error("expected 'git worktree prune' NOT to be invoked when repair+remove succeed")
	}
}

func TestRemoveCandidatesProgressCallbacks(t *testing.T) {
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	var messages []string
	onProgress := progress.Func(func(msg string) { messages = append(messages, msg) })

	candidates := []CleanCandidate{
		{Path: "/wt/a", Branch: "feature/a"},
		{Path: "/wt/b", Branch: "feature/b"},
	}

	RemoveCandidates(context.Background(), r, candidates, false, false, onProgress)
	if len(messages) != 2 {
		t.Fatalf("expected 2 progress messages, got %d", len(messages))
	}
}

func TestRemoveCandidatesWritesOneManifestForWholeSweep(t *testing.T) {
	commonDir := t.TempDir()
	var manifestSightings int
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "rev-parse --git-common-dir"):
				return commonDir, nil
			case strings.Contains(cmd, "worktree remove"):
				matches, _ := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
				if len(matches) == 1 {
					manifestSightings++
				}
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates := []CleanCandidate{
		{Path: "/wt/a", Branch: "feature/a"},
		{Path: "/wt/b", Branch: "feature/b"},
		{Path: "/wt/c", Branch: "feature/c"},
	}

	RemoveCandidates(context.Background(), r, candidates, false, false, nil)

	if manifestSightings != len(candidates) {
		t.Errorf("manifest sightings = %d, want %d (one manifest covering the whole sweep, seen at every removal)", manifestSightings, len(candidates))
	}
	matches, _ := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if len(matches) != 0 {
		t.Errorf("expected manifest cleaned up after the sweep completes, got %v", matches)
	}
}

func TestFindMergedCandidatesSquashMergeError(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", branchMain},
		struct{ path, branch string }{"/wt/active", "feature/active"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// MergedBranches: return empty (no regular merges)
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "", nil
			}
			// ListWorktrees
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			// IsSquashMerged requires merge-base — fail on it
			if len(args) > 0 && args[0] == "merge-base" {
				return "", errors.New("merge-base failed")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", branchMain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The entry should be skipped (squash merge check errored), so no candidates
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}
	// But we should get a warning about the skipped branch
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestFindStaleCandidatesLastCommitError(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", branchMain},
		struct{ path, branch string }{"/wt/broken", "feature/broken"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// ListWorktrees
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			// LastCommitTime: log -1 --format=%ct <branch>
			if len(args) > 0 && args[0] == gitCmdLog {
				return "", errors.New("no commits")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindStaleCandidates(context.Background(), r, branchMain, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Entry should be skipped due to error, so no candidates
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}
	// But we should get a warning about the skipped branch
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(result.Warnings))
	}
}

// TestFindMergedCandidatesMergedBranchOwnCommitsError checks that a failed
// mainline check skips the branch with a warning instead of erroring out.
func TestFindMergedCandidatesMergedBranchOwnCommitsError(t *testing.T) {
	wt := porcelainEntries(
		struct{ path, branch string }{"/repo", branchMain},
		struct{ path, branch string }{"/wt/active", "feature/active"},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// MergedBranches: feature/active reported as merged
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "  feature/active\n", nil
			}
			// ListWorktrees
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			// FirstParentChainSHAs fails, so the mainline check is skipped with a warning.
			if len(args) > 0 && args[0] == gitCmdRevList {
				return "", errors.New("rev-list failed")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(context.Background(), r, "origin/main", branchMain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates (own-commits error must skip branch), got %d", len(result.Candidates))
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning for skipped branch, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestFindMergedCandidatesListWorktreesError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdBranch {
				return "", nil // MergedBranches succeeds (empty list)
			}
			return "", errors.New("worktree list failed")
		},
		runInDir: noopRunInDir,
	}

	_, err := FindMergedCandidates(context.Background(), r, "origin/main", branchMain)
	if err == nil {
		t.Fatal("expected error when ListWorktrees fails")
	}
}

func TestRemoveCandidatesRemoteDeleteSuccess(t *testing.T) {
	pushCalled := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdPush {
				pushCalled = true
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates := []CleanCandidate{{Path: "/wt/a", Branch: "feature/a"}}
	items := RemoveCandidates(context.Background(), r, candidates, true, false, nil) // originPresent=true passed by caller
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].RemoteDeleted {
		t.Error("expected RemoteDeleted=true")
	}
	if items[0].RemoteError != nil {
		t.Errorf("expected nil RemoteError, got: %v", items[0].RemoteError)
	}
	if !pushCalled {
		t.Error("expected git push --delete to be called")
	}
}

func TestRemoveCandidatesRemoteDeleteFailureStillCountsItem(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdPush {
				return "", errors.New("connection refused")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates := []CleanCandidate{{Path: "/wt/a", Branch: "feature/a"}}
	items := RemoveCandidates(context.Background(), r, candidates, true, false, nil)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !items[0].WorktreeRemoved {
		t.Error("expected WorktreeRemoved=true")
	}
	if items[0].RemoteDeleted {
		t.Error("expected RemoteDeleted=false on failure")
	}
	if items[0].RemoteError == nil {
		t.Error("expected non-nil RemoteError")
	}
}

func TestRemoveCandidatesNoOriginSkipsRemoteDelete(t *testing.T) {
	// Caller resolved RemoteExists=false and passes originPresent=false.
	pushCalled := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdPush {
				pushCalled = true
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates := []CleanCandidate{{Path: "/wt/a", Branch: "feature/a"}}
	items := RemoveCandidates(context.Background(), r, candidates, false, false, nil) // originPresent=false → no push
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].RemoteDeleted {
		t.Error("expected RemoteDeleted=false when originPresent=false")
	}
	if pushCalled {
		t.Error("expected no push call when originPresent=false")
	}
}

func TestRemoveCandidatesForceFlag(t *testing.T) {
	tests := []struct {
		name      string
		force     bool
		wantForce bool
	}{
		{name: "force=true passes --force to git", force: true, wantForce: true},
		{name: "force=false omits --force from git", force: false, wantForce: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			r := &mockRunner{
				run: func(args ...string) (string, error) {
					if len(args) > 0 && args[0] == gitCmdWorktree {
						capturedArgs = args
					}
					return "", nil
				},
				runInDir: noopRunInDir,
			}
			candidates := []CleanCandidate{{Path: "/wt/a", Branch: "feature/a"}}
			RemoveCandidates(context.Background(), r, candidates, false, tt.force, nil)
			hasForce := slices.Contains(capturedArgs, "--force")
			if hasForce != tt.wantForce {
				t.Errorf("git worktree args %v: --force present=%v, want %v", capturedArgs, hasForce, tt.wantForce)
			}
		})
	}
}
