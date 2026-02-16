package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteWorktreeTasks(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		wtFeatureLogin,
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
		if !found[taskLogin] {
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
		if tasks[0] != taskLogin {
			t.Errorf("task = %q, want %q", tasks[0], taskLogin)
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

func TestCompleteWorktreeTasksSkipsBare(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"bare",
		"",
		wtFeatureLogin,
		"HEAD def456",
		"branch refs/heads/" + branchFeature,
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	tasks := completeWorktreeTasks(cmd, "")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (bare filtered), got %d: %v", len(tasks), tasks)
	}
	if tasks[0] != taskLogin {
		t.Errorf("task = %q, want %q", tasks[0], taskLogin)
	}
}

func TestCompleteBranchNamesError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}

	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	branches := completeBranchNames(cmd, "")
	if branches != nil {
		t.Errorf("expected nil branches on error, got %v", branches)
	}
}

func TestListTypeFlagCompletion(t *testing.T) {
	fn, ok := listCmd.GetFlagCompletionFunc(flagType)
	if !ok {
		t.Fatal("no completion function registered for --type flag")
	}

	t.Run("all types", func(t *testing.T) {
		types, directive := fn(listCmd, nil, "")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(types) == 0 {
			t.Fatal("expected at least one type completion")
		}
		found := map[string]bool{}
		for _, tp := range types {
			found[tp] = true
		}
		if !found["feature"] {
			t.Error("expected 'feature' in completions")
		}
		if !found[benchFilterType] {
			t.Error("expected 'bugfix' in completions")
		}
	})

	t.Run("filter by prefix", func(t *testing.T) {
		types, _ := fn(listCmd, nil, "bug")
		if len(types) != 1 {
			t.Fatalf("expected 1 type, got %d: %v", len(types), types)
		}
		if types[0] != benchFilterType {
			t.Errorf("type = %q, want %q", types[0], benchFilterType)
		}
	})

	t.Run("no match", func(t *testing.T) {
		types, _ := fn(listCmd, nil, "zzz")
		if len(types) != 0 {
			t.Errorf("expected 0 types for 'zzz', got %d: %v", len(types), types)
		}
	})
}

func TestAddSourceFlagCompletion(t *testing.T) {
	fn, ok := addCmd.GetFlagCompletionFunc(flagSource)
	if !ok {
		t.Fatal("no completion function registered for --source flag")
	}

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "main\ndevelop\nstaging\n", nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	t.Run("all branches", func(t *testing.T) {
		branches, directive := fn(addCmd, nil, "")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(branches) != 3 {
			t.Fatalf("expected 3 branches, got %d: %v", len(branches), branches)
		}
	})

	t.Run("filter by prefix", func(t *testing.T) {
		branches, _ := fn(addCmd, nil, "dev")
		if len(branches) != 1 {
			t.Fatalf("expected 1 branch, got %d: %v", len(branches), branches)
		}
		if branches[0] != "develop" {
			t.Errorf("branch = %q, want %q", branches[0], "develop")
		}
	})
}

func TestMergeIntoFlagCompletion(t *testing.T) {
	fn, ok := mergeCmd.GetFlagCompletionFunc(flagInto)
	if !ok {
		t.Fatal("no completion function registered for --into flag")
	}

	porcelain := strings.Join([]string{
		wtRepo,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	t.Run("all tasks", func(t *testing.T) {
		tasks, directive := fn(mergeCmd, nil, "")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
		}
		if len(tasks) < 1 {
			t.Fatalf("expected at least 1 task, got %d", len(tasks))
		}
	})

	t.Run("filter by prefix", func(t *testing.T) {
		tasks, _ := fn(mergeCmd, nil, "log")
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
		}
		if tasks[0] != taskLogin {
			t.Errorf("task = %q, want %q", tasks[0], taskLogin)
		}
	})
}
