package operations

import (
	"errors"
	"strings"
	"testing"
)

const (
	gitCmdBranch   = "branch"
	gitCmdLog      = "log"
	gitCmdWorktree = "worktree"
	gitSubcmdAdd   = "add"
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

func TestFindMergedCandidates_NormalMerge(t *testing.T) {
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
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(r, "origin/main", "main")
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

func TestFindMergedCandidates_SquashMerge(t *testing.T) {
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
			// IsSquashMerged: merge-base → rev-parse → commit-tree → cherry
			if len(args) > 0 && args[0] == "merge-base" {
				return "base123", nil
			}
			if len(args) > 0 && args[0] == "rev-parse" {
				return "tree123", nil
			}
			if len(args) > 0 && args[0] == "commit-tree" {
				return "temp123", nil
			}
			if len(args) > 0 && args[0] == "cherry" {
				return "- temp123", nil // "- " prefix = already merged
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := FindMergedCandidates(r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
}

func TestFindMergedCandidates_NoCandidates(t *testing.T) {
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

	result, err := FindMergedCandidates(r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}
}

func TestFindMergedCandidates_GitError(t *testing.T) {
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", errors.New("git failed") },
		runInDir: noopRunInDir,
	}

	_, err := FindMergedCandidates(r, "origin/main", "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindStaleCandidates_Found(t *testing.T) {
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

	result, err := FindStaleCandidates(r, "main", 14)
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

func TestFindStaleCandidates_NoneStale(t *testing.T) {
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

	result, err := FindStaleCandidates(r, "main", 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}
}

func TestFindStaleCandidates_GitError(t *testing.T) {
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", errors.New("git failed") },
		runInDir: noopRunInDir,
	}

	_, err := FindStaleCandidates(r, "main", 14)
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

func TestRemoveCandidates_MixedResults(t *testing.T) {
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
		{Path: "/wt/b", Branch: "feature/b"}, // Will fail removal
		{Path: "/wt/c", Branch: "feature/c"},
	}

	items := RemoveCandidates(r, candidates, nil)
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

func TestRemoveCandidates_ProgressCallbacks(t *testing.T) {
	r := &mockRunner{
		run:      func(args ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	var messages []string
	progress := ProgressFunc(func(msg string) { messages = append(messages, msg) })

	candidates := []CleanCandidate{
		{Path: "/wt/a", Branch: "feature/a"},
		{Path: "/wt/b", Branch: "feature/b"},
	}

	RemoveCandidates(r, candidates, progress)
	if len(messages) != 2 {
		t.Fatalf("expected 2 progress messages, got %d", len(messages))
	}
}

func TestFindMergedCandidates_SquashMergeError(t *testing.T) {
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

	result, err := FindMergedCandidates(r, "origin/main", branchMain)
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

func TestFindStaleCandidates_LastCommitError(t *testing.T) {
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

	result, err := FindStaleCandidates(r, branchMain, 14)
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
