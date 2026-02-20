package mcp

import (
	"errors"
	"strings"
	"testing"
)

const (
	gitDiff      = "diff"
	gitMergeTree = "merge-tree"
	diffNameOnly = "--name-only"
	writeTree    = "--write-tree"
	fileShared   = "shared.go"
)

func TestConflictCheckToolRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestConflictCheckToolNoEligible(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[conflictCheckData](t, result)
	if len(data.Overlaps) != 0 {
		t.Errorf("expected 0 overlaps, got %d", len(data.Overlaps))
	}
}

func TestConflictCheckToolOverlapsDetected(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			// CollectDiffs: git diff --name-only main...branch
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				ref := args[2]
				if strings.Contains(ref, "feature/task-a") {
					return "shared.go\nonly-a.go", nil
				}
				if strings.Contains(ref, "feature/task-b") {
					return "shared.go\nonly-b.go", nil
				}
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[conflictCheckData](t, result)

	if len(data.Overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(data.Overlaps))
	}
	overlap := data.Overlaps[0]
	if overlap.File != fileShared {
		t.Errorf("expected overlapping file 'shared.go', got %q", overlap.File)
	}
	if len(overlap.Branches) != 2 {
		t.Errorf("expected 2 branches in overlap, got %d", len(overlap.Branches))
	}
	if overlap.Severity != "low" {
		t.Errorf("expected severity 'low' for 2-branch overlap, got %q", overlap.Severity)
	}
	if data.TotalFiles != 3 {
		t.Errorf("expected 3 total files, got %d", data.TotalFiles)
	}
	if data.TotalBranches != 2 {
		t.Errorf("expected 2 total branches, got %d", data.TotalBranches)
	}
}

func TestConflictCheckToolHighSeverity(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-c", "feature/task-c"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				// All three branches touch fileShared
				return fileShared, nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[conflictCheckData](t, result)

	if len(data.Overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(data.Overlaps))
	}
	if data.Overlaps[0].Severity != "high" {
		t.Errorf("expected severity 'high' for 3+ branch overlap, got %q", data.Overlaps[0].Severity)
	}
	if len(data.Overlaps[0].Branches) != 3 {
		t.Errorf("expected 3 branches in overlap, got %d", len(data.Overlaps[0].Branches))
	}
}

func TestConflictCheckToolNoOverlaps(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				ref := args[2]
				if strings.Contains(ref, "feature/task-a") {
					return "only-a.go", nil
				}
				if strings.Contains(ref, "feature/task-b") {
					return "only-b.go", nil
				}
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[conflictCheckData](t, result)

	if len(data.Overlaps) != 0 {
		t.Errorf("expected 0 overlaps, got %d", len(data.Overlaps))
	}
	if data.TotalFiles != 2 {
		t.Errorf("expected 2 total files, got %d", data.TotalFiles)
	}
}

func TestConflictCheckToolDryMerge(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			// CollectDiffs
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				return fileShared, nil
			}
			// DryMergeAll: git merge-tree --write-tree branch1 branch2
			if len(args) >= 3 && args[0] == gitMergeTree && args[1] == writeTree {
				// Return conflict output
				return "CONFLICT (content): Merge conflict in shared.go", errors.New("exit status 1")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, map[string]any{"dry_merge": true})
	data := unmarshalJSON[conflictCheckData](t, result)

	if len(data.Overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(data.Overlaps))
	}
	if len(data.DryMerges) != 1 {
		t.Fatalf("expected 1 dry merge result, got %d", len(data.DryMerges))
	}
	dm := data.DryMerges[0]
	if !dm.HasConflicts {
		t.Errorf("expected dry merge to have conflicts")
	}
	if len(dm.ConflictFiles) != 1 || dm.ConflictFiles[0] != fileShared {
		t.Errorf("expected conflict file 'shared.go', got %v", dm.ConflictFiles)
	}
}

func TestConflictCheckToolDryMergeNoConflicts(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				return fileShared, nil
			}
			// merge-tree succeeds (no conflicts)
			if len(args) >= 3 && args[0] == gitMergeTree && args[1] == writeTree {
				return "abc123def456", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, map[string]any{"dry_merge": true})
	data := unmarshalJSON[conflictCheckData](t, result)

	if len(data.DryMerges) != 1 {
		t.Fatalf("expected 1 dry merge result, got %d", len(data.DryMerges))
	}
	if data.DryMerges[0].HasConflicts {
		t.Errorf("expected no conflicts in dry merge")
	}
}

func TestConflictCheckToolCollectDiffsError(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				return "", errors.New("diff failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "diff") {
		t.Errorf("expected diff error, got: %s", errText)
	}
}

func TestConflictCheckToolDryMergeError(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				return "file.go", nil
			}
			// merge-tree returns an error with no CONFLICT lines (genuine error)
			if len(args) >= 3 && args[0] == gitMergeTree && args[1] == writeTree {
				return "", errors.New("merge-tree failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, map[string]any{"dry_merge": true})
	errText := resultError(t, result)
	if !strings.Contains(errText, "merge-tree") {
		t.Errorf("expected merge-tree error, got: %s", errText)
	}
}

func TestConflictCheckToolListWorktreesFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return "", errors.New("worktree list failed")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "worktree list failed") {
		t.Errorf("expected worktree list error, got: %s", errText)
	}
}

func TestConflictCheckToolSingleWorktreeNoOverlaps(t *testing.T) {
	// Single eligible worktree can't have overlaps with itself
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				return "file.go", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[conflictCheckData](t, result)

	if len(data.Overlaps) != 0 {
		t.Errorf("expected 0 overlaps with single worktree, got %d", len(data.Overlaps))
	}
	if data.TotalBranches != 1 {
		t.Errorf("expected 1 total branch, got %d", data.TotalBranches)
	}
}

func TestConflictCheckToolMultipleOverlappingFiles(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-a", "feature/task-a"},
		struct{ path, branch string }{"/repo/.worktrees/feature-task-b", "feature/task-b"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) >= 3 && args[0] == gitDiff && args[1] == diffNameOnly {
				ref := args[2]
				if strings.Contains(ref, "feature/task-a") {
					return "shared1.go\nshared2.go\nonly-a.go", nil
				}
				if strings.Contains(ref, "feature/task-b") {
					return "shared1.go\nshared2.go\nonly-b.go", nil
				}
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleConflictCheck(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[conflictCheckData](t, result)

	if len(data.Overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %d", len(data.Overlaps))
	}
	// Overlaps should be sorted alphabetically (both low severity)
	if data.Overlaps[0].File != "shared1.go" {
		t.Errorf("expected first overlap 'shared1.go', got %q", data.Overlaps[0].File)
	}
	if data.Overlaps[1].File != "shared2.go" {
		t.Errorf("expected second overlap 'shared2.go', got %q", data.Overlaps[1].File)
	}
}
