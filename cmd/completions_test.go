package cmd

import (
	"strings"
	"testing"
)

func TestCompleteWorktreeTasks(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /wt/feature-login",
		"HEAD def456",
		"branch refs/heads/" + branchFeature,
		"",
		"worktree /wt/bugfix-typo",
		"HEAD ghi789",
		"branch refs/heads/bugfix/typo",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()

	t.Run("complete all", func(t *testing.T) {
		tasks := completeWorktreeTasks(cmd, "")
		if len(tasks) < 2 {
			t.Fatalf("expected at least 2 tasks, got %d: %v", len(tasks), tasks)
		}
		found := map[string]bool{}
		for _, task := range tasks {
			found[task] = true
		}
		if !found["login"] {
			t.Error("expected 'login' in completions")
		}
		if !found["typo"] {
			t.Error("expected 'typo' in completions")
		}
	})

	t.Run("complete with prefix filter", func(t *testing.T) {
		tasks := completeWorktreeTasks(cmd, "log")
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
		}
		if tasks[0] != "login" {
			t.Errorf("task = %q, want %q", tasks[0], "login")
		}
	})
}

func TestCompleteBranchNames(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "main\nfeature/login\nbugfix/typo\n", nil },
		runInDir: noopRunInDir,
	}

	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()

	t.Run("complete all", func(t *testing.T) {
		branches := completeBranchNames(cmd, "")
		if len(branches) != 3 {
			t.Fatalf("expected 3 branches, got %d: %v", len(branches), branches)
		}
	})

	t.Run("complete with prefix filter", func(t *testing.T) {
		branches := completeBranchNames(cmd, "feature")
		if len(branches) != 1 {
			t.Fatalf("expected 1 branch, got %d: %v", len(branches), branches)
		}
		if branches[0] != branchFeature {
			t.Errorf("branch = %q, want %q", branches[0], branchFeature)
		}
	})
}

func TestCompleteWorktreeTasksError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}

	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	tasks := completeWorktreeTasks(cmd, "")
	if tasks != nil {
		t.Errorf("expected nil tasks on error, got %v", tasks)
	}
}
