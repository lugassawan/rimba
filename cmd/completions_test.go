package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
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
		branchRefPrefix + branchFeature,
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
			t.Errorf(taskWantFmt, tasks[0], taskLogin)
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
		branchRefPrefix + branchFeature,
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
		t.Errorf(taskWantFmt, tasks[0], taskLogin)
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

// assertContainsAll verifies that items contains every entry in want.
func assertContainsAll(t *testing.T, items []string, want ...string) {
	t.Helper()
	found := map[string]bool{}
	for _, item := range items {
		found[item] = true
	}
	for _, w := range want {
		if !found[w] {
			t.Errorf("expected %q in completions %v", w, items)
		}
	}
}

func TestCompleteOpenShortcuts(t *testing.T) {
	cfg := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: "main",
		Open: map[string]string{
			"ide":   "code .",
			"agent": "claude",
			"test":  "npm test",
		},
	}

	t.Run("returns all config keys", func(t *testing.T) {
		cmd, _ := newTestCmd()
		cmd.SetContext(config.WithConfig(context.Background(), cfg))
		names := completeOpenShortcuts(cmd, "")
		if len(names) != 3 {
			t.Fatalf("expected 3 shortcuts, got %d: %v", len(names), names)
		}
		// Should be sorted
		if names[0] != "agent" || names[1] != "ide" || names[2] != "test" {
			t.Errorf("names = %v, want [agent ide test]", names)
		}
	})

	t.Run("filters by prefix", func(t *testing.T) {
		cmd, _ := newTestCmd()
		cmd.SetContext(config.WithConfig(context.Background(), cfg))
		names := completeOpenShortcuts(cmd, "i")
		if len(names) != 1 {
			t.Fatalf("expected 1 shortcut, got %d: %v", len(names), names)
		}
		if names[0] != "ide" {
			t.Errorf("name = %q, want %q", names[0], "ide")
		}
	})

	t.Run("nil config returns nil", func(t *testing.T) {
		cmd, _ := newTestCmd()
		cmd.SetContext(context.Background())
		names := completeOpenShortcuts(cmd, "")
		if names != nil {
			t.Errorf("expected nil for no config, got %v", names)
		}
	})

	t.Run("nil open section returns nil", func(t *testing.T) {
		cfgNoOpen := &config.Config{WorktreeDir: "../wt", DefaultSource: "main"}
		cmd, _ := newTestCmd()
		cmd.SetContext(config.WithConfig(context.Background(), cfgNoOpen))
		names := completeOpenShortcuts(cmd, "")
		if names != nil {
			t.Errorf("expected nil for nil Open, got %v", names)
		}
	})
}

func TestOpenWithFlagCompletion(t *testing.T) {
	fn, ok := openCmd.GetFlagCompletionFunc(flagWith)
	if !ok {
		t.Fatal("no completion function registered for --with flag")
	}

	cfg := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: "main",
		Open:          map[string]string{"ide": "code .", "test": "npm test"},
	}
	ctx := config.WithConfig(context.Background(), cfg)
	openCmd.SetContext(ctx)
	t.Cleanup(func() { openCmd.SetContext(context.TODO()) })

	names, directive := fn(openCmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf(directiveWantFmt, directive)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 shortcuts, got %d: %v", len(names), names)
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
			t.Errorf(directiveWantFmt, directive)
		}
		if len(types) == 0 {
			t.Fatal("expected at least one type completion")
		}
		assertContainsAll(t, types, "feature", benchFilterType)
	})

	t.Run(filterByPrefix, func(t *testing.T) {
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
			t.Errorf(directiveWantFmt, directive)
		}
		if len(branches) != 3 {
			t.Fatalf("expected 3 branches, got %d: %v", len(branches), branches)
		}
	})

	t.Run(filterByPrefix, func(t *testing.T) {
		branches, _ := fn(addCmd, nil, "dev")
		if len(branches) != 1 {
			t.Fatalf("expected 1 branch, got %d: %v", len(branches), branches)
		}
		if branches[0] != branchDevelop {
			t.Errorf("branch = %q, want %q", branches[0], branchDevelop)
		}
	})
}

func TestCompleteArchivedTasks(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == cmdRevParse && args[1] == cmdShowToplevel:
				return repoPath, nil
			case args[0] == cmdSymbolicRef:
				return refsRemotesOriginMain, nil
			case args[0] == cmdBranch:
				return branchListArchived, nil
			case args[0] == cmdWorktreeTest && args[1] == cmdList:
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					"worktree /wt/feature-active-task", "HEAD def456", "branch refs/heads/feature/active-task", "",
				}, "\n"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()

	t.Run("returns archived tasks", func(t *testing.T) {
		tasks := completeArchivedTasks(cmd, "")
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
		}
		if tasks[0] != "archived-task" {
			t.Errorf("task = %q, want %q", tasks[0], "archived-task")
		}
	})

	t.Run("filters by prefix", func(t *testing.T) {
		tasks := completeArchivedTasks(cmd, "arch")
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
		}
	})

	t.Run("no match", func(t *testing.T) {
		tasks := completeArchivedTasks(cmd, "zzz")
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks, got %d: %v", len(tasks), tasks)
		}
	})
}

func TestCompleteArchivedTasksError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdRevParse {
				return repoPath, nil
			}
			if args[0] == cmdSymbolicRef {
				return refsRemotesOriginMain, nil
			}
			if args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	tasks := completeArchivedTasks(cmd, "")
	if tasks != nil {
		t.Errorf("expected nil tasks on error, got %v", tasks)
	}
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
			t.Errorf(directiveWantFmt, directive)
		}
		if len(tasks) < 1 {
			t.Fatalf("expected at least 1 task, got %d", len(tasks))
		}
	})

	t.Run(filterByPrefix, func(t *testing.T) {
		tasks, _ := fn(mergeCmd, nil, "log")
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d: %v", len(tasks), tasks)
		}
		if tasks[0] != taskLogin {
			t.Errorf(taskWantFmt, tasks[0], taskLogin)
		}
	})
}
