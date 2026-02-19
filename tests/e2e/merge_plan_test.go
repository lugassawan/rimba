package e2e_test

import (
	"strings"
	"testing"
)

func TestMergePlanNoWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)
	r := rimbaSuccess(t, repo, "merge-plan")
	assertContains(t, r.Stdout, "No active worktree branches")
}

func TestMergePlanOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupCleanInitializedRepo(t)

	// A and B share a file, C does not overlap
	conflictSetup(t, repo, taskConflictA, "shared.txt", "content from a")
	conflictSetup(t, repo, taskConflictB, "shared.txt", "content from b")
	conflictSetup(t, repo, taskConflictC, "unique-c.txt", "content from c")

	r := rimbaSuccess(t, repo, "merge-plan")
	assertContains(t, r.Stdout, "Merge in this order")

	// C should appear first (fewest conflicts)
	lines := strings.Split(r.Stdout, "\n")
	var dataLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' {
			dataLines = append(dataLines, trimmed)
		}
	}

	if len(dataLines) < 3 {
		t.Fatalf("expected at least 3 data lines, got %d:\n%s", len(dataLines), r.Stdout)
	}

	// First line should contain cc-task-c (fewest conflicts = 0)
	if !strings.Contains(dataLines[0], taskConflictC) {
		t.Errorf("first merge should be %s, got: %s", taskConflictC, dataLines[0])
	}
}
