package operations

import (
	"testing"
)

const (
	cmdBranch       = "branch"
	cmdWorktreeTest = "worktree"
	cmdList         = "list"

	branchListArchived = "main\nfeature/archived-task\nfeature/active-task"
)

func TestFindArchivedBranch(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return branchListArchived, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
					"worktree /wt/feature-active-task\nHEAD abc\nbranch refs/heads/feature/active-task\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	branch, err := FindArchivedBranch(mr, "archived-task")
	if err != nil {
		t.Fatalf("FindArchivedBranch: %v", err)
	}
	if branch != "feature/archived-task" {
		t.Errorf("branch = %q, want %q", branch, "feature/archived-task")
	}
}

func TestFindArchivedBranchNotFound(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nfeature/active-task", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
					"worktree /wt/feature-active-task\nHEAD abc\nbranch refs/heads/feature/active-task\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := FindArchivedBranch(mr, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent archived branch")
	}
}

func TestFindArchivedBranchExactMatch(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nmy-custom-branch", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	branch, err := FindArchivedBranch(mr, "my-custom-branch")
	if err != nil {
		t.Fatalf("FindArchivedBranch: %v", err)
	}
	if branch != "my-custom-branch" {
		t.Errorf("branch = %q, want %q", branch, "my-custom-branch")
	}
}

func TestFindArchivedBranchByTaskExtraction(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nbugfix/some-task", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	branch, err := FindArchivedBranch(mr, "some-task")
	if err != nil {
		t.Fatalf("FindArchivedBranch: %v", err)
	}
	if branch != "bugfix/some-task" {
		t.Errorf("branch = %q, want %q", branch, "bugfix/some-task")
	}
}

func TestFindArchivedBranchExactMatchSkipsActive(t *testing.T) {
	// Branch name matches task exactly but is active → falls through to task extraction
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				// "my-task" is an exact match, "bugfix/my-task" matches via extraction
				return "main\nmy-task\nbugfix/my-task", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				// "my-task" is active (checked out), "bugfix/my-task" is not
				return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
					"worktree /wt/my-task\nHEAD def\nbranch refs/heads/my-task\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	branch, err := FindArchivedBranch(mr, "my-task")
	if err != nil {
		t.Fatalf("FindArchivedBranch: %v", err)
	}
	// Should skip "my-task" (active) and find "bugfix/my-task" via task extraction
	if branch != "bugfix/my-task" {
		t.Errorf("branch = %q, want %q", branch, "bugfix/my-task")
	}
}

func TestFindArchivedBranchPrefixMatchSkipsActive(t *testing.T) {
	// Prefix+task match exists but is active → falls through to exact match
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nfeature/task-x\ntask-x", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				// "feature/task-x" is active
				return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
					"worktree /wt/feature-task-x\nHEAD def\nbranch refs/heads/feature/task-x\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	branch, err := FindArchivedBranch(mr, "task-x")
	if err != nil {
		t.Fatalf("FindArchivedBranch: %v", err)
	}
	// Should skip "feature/task-x" (active) and find "task-x" via exact match
	if branch != "task-x" {
		t.Errorf("branch = %q, want %q", branch, "task-x")
	}
}

func TestFindArchivedBranchSkipsActiveInFallback(t *testing.T) {
	// Branch "bugfix/some-task" matches task "some-task" via extraction,
	// but it's active in a worktree → should be skipped in fallback.
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nbugfix/some-task", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n" +
					"worktree /wt/bugfix-some-task\nHEAD def\nbranch refs/heads/bugfix/some-task\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := FindArchivedBranch(mr, "some-task")
	if err == nil {
		t.Fatal("expected error when only matching branch is active")
	}
}

func TestFindArchivedBranchError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := FindArchivedBranch(mr, "any")
	if err == nil {
		t.Fatal("expected error from LocalBranches failure")
	}
}

func TestFindArchivedBranchWorktreeError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "main\nfeature/task", nil
			}
			if args[0] == cmdWorktreeTest {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := FindArchivedBranch(mr, "task")
	if err == nil {
		t.Fatal("expected error from ListWorktrees failure")
	}
}

func TestListArchivedBranches(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdBranch:
				return "main\nfeature/archived\nfeature/active", nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
					"worktree /wt/feature-active\nHEAD def456\nbranch refs/heads/feature/active\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	archived, err := ListArchivedBranches(mr, branchMain)
	if err != nil {
		t.Fatalf("ListArchivedBranches: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("got %d branches, want 1", len(archived))
	}
	if archived[0] != "feature/archived" {
		t.Errorf("branch = %q, want %q", archived[0], "feature/archived")
	}
}

func TestListArchivedBranchesError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := ListArchivedBranches(mr, branchMain)
	if err == nil {
		t.Fatal("expected error from LocalBranches failure")
	}
}

func TestListArchivedBranchesWorktreeError(t *testing.T) {
	mr := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "main\nfeature/task", nil
			}
			if args[0] == cmdWorktreeTest {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := ListArchivedBranches(mr, branchMain)
	if err == nil {
		t.Fatal("expected error from ListWorktrees failure")
	}
}
