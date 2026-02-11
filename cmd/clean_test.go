package cmd

import (
	"errors"
	"strings"
	"testing"
)

// mergedWorktreeRunner returns a mockRunner that supports MergedBranches and ListWorktrees.
func mergedWorktreeRunner(mergedOut, worktreeOut string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdBranch {
				return mergedOut, nil
			}
			if len(args) >= 1 && args[0] == cmdWorktreeTest {
				return worktreeOut, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

func TestFindMergedCandidatesFound(t *testing.T) {
	worktreeOut := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree " + pathWtDone,
		"HEAD def456",
		"branch refs/heads/" + branchDone,
		"",
		"worktree /wt/feature-active",
		"HEAD ghi789",
		"branch refs/heads/feature/active",
		"",
	}, "\n")

	r := mergedWorktreeRunner("  "+branchDone+"\n  bugfix/old", worktreeOut)
	candidates, err := findMergedCandidates(r, branchMain)
	if err != nil {
		t.Fatalf("findMergedCandidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(candidates))
	}
	if candidates[0].branch != branchDone {
		t.Errorf("branch = %q, want %q", candidates[0].branch, branchDone)
	}
}

func TestFindMergedCandidatesNone(t *testing.T) {
	r := mergedWorktreeRunner("", "worktree /repo\nHEAD abc\nbranch refs/heads/main\n")
	candidates, err := findMergedCandidates(r, branchMain)
	if err != nil {
		t.Fatalf("findMergedCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestFindMergedCandidatesError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := findMergedCandidates(r, branchMain)
	if err == nil {
		t.Fatal(errExpected)
	}
}

func TestPrintMergedCandidates(t *testing.T) {
	cmd, buf := newTestCmd()
	candidates := []mergedCandidate{
		{path: pathWtDone, branch: branchDone},
		{path: "/wt/bugfix-old", branch: "bugfix/old"},
	}
	printMergedCandidates(cmd, candidates)

	out := buf.String()
	if !strings.Contains(out, "Merged worktrees:") {
		t.Errorf("output missing header: %q", out)
	}
	if !strings.Contains(out, "done") {
		t.Errorf("output missing task 'done': %q", out)
	}
	if !strings.Contains(out, "old") {
		t.Errorf("output missing task 'old': %q", out)
	}
}

func testMergedCandidate() []mergedCandidate {
	return []mergedCandidate{{path: pathWtDone, branch: branchDone}}
}

func TestRemoveMergedWorktreesAllSucceed(t *testing.T) {
	cmd, _ := newTestCmd()
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	removed := removeMergedWorktrees(cmd, r, testMergedCandidate())
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
}

func TestRemoveMergedWorktreesRemoveFails(t *testing.T) {
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == "remove" {
				return "", errors.New("locked")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	removed := removeMergedWorktrees(cmd, r, testMergedCandidate())
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if !strings.Contains(buf.String(), "Failed to remove") {
		t.Errorf("output = %q, want failure message", buf.String())
	}
}

func TestRemoveMergedWorktreesDeleteBranchFails(t *testing.T) {
	cmd, buf := newTestCmd()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdBranch {
				return "", errors.New("branch not found")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	removed := removeMergedWorktrees(cmd, r, testMergedCandidate())
	if removed != 0 {
		t.Errorf("removed = %d, want 0 (branch delete failed)", removed)
	}
	if !strings.Contains(buf.String(), "failed to delete branch") {
		t.Errorf("output = %q, want branch delete failure message", buf.String())
	}
}

func TestConfirmRemoval(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes_short", "y\n", true},
		{"yes_full", "yes\n", true},
		{"no", "n\n", false},
		{"empty", "\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _ := newTestCmd()
			cmd.SetIn(strings.NewReader(tt.input))
			got := confirmRemoval(cmd, 2)
			if got != tt.want {
				t.Errorf("confirmRemoval(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
