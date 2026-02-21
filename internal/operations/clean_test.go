package operations

import (
	"errors"
	"strings"
	"testing"
)

const (
	gitCmdBranch   = "branch"
	gitCmdWorktree = "worktree"
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

	candidates, err := FindMergedCandidates(r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Branch != "feature/done" {
		t.Errorf("expected feature/done, got %s", candidates[0].Branch)
	}
	if candidates[1].Branch != "bugfix/fixed" {
		t.Errorf("expected bugfix/fixed, got %s", candidates[1].Branch)
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

	candidates, err := FindMergedCandidates(r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
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

	candidates, err := FindMergedCandidates(r, "origin/main", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
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
			if len(args) > 0 && args[0] == "log" {
				// Unix epoch for 2020-01-01 with a tab-separated subject
				return "1577836800\told commit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates, err := FindStaleCandidates(r, "main", 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Branch != "feature/old" {
		t.Errorf("expected feature/old, got %s", candidates[0].Branch)
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
			if len(args) > 0 && args[0] == "log" {
				// Unix epoch for 2099-01-01 with a tab-separated subject
				return "4070908800\tfresh commit", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	candidates, err := FindStaleCandidates(r, "main", 14)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
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

	// First: success
	if !items[0].WorktreeRemoved || !items[0].BranchDeleted {
		t.Errorf("item 0: expected removed+deleted, got wt=%v br=%v", items[0].WorktreeRemoved, items[0].BranchDeleted)
	}
	// Second: failed removal
	if items[1].WorktreeRemoved || items[1].BranchDeleted {
		t.Errorf("item 1: expected not removed, got wt=%v br=%v", items[1].WorktreeRemoved, items[1].BranchDeleted)
	}
	// Third: success
	if !items[2].WorktreeRemoved || !items[2].BranchDeleted {
		t.Errorf("item 2: expected removed+deleted, got wt=%v br=%v", items[2].WorktreeRemoved, items[2].BranchDeleted)
	}
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
