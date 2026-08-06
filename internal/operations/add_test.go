package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/gitref"
	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/progress"
)

func TestAddWorktreeSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse --verify returns error = doesn't exist
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			// AddWorktree: create the directory
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Task != "login" {
		t.Errorf("expected task 'login', got %q", result.Task)
	}
	if result.Branch != "feature/login" {
		t.Errorf("expected branch 'feature/login', got %q", result.Branch)
	}
	if result.Source != branchMain {
		t.Errorf("expected source 'main', got %q", result.Source)
	}
	if !strings.Contains(result.Path, ".worktrees") {
		t.Errorf("expected path to contain .worktrees, got %q", result.Path)
	}
}

// TestAddWorktreeRecordsCreateSpan verifies AddWorktree wraps git.AddWorktree
// in a "create" span when a Recorder is attached to ctx.
func TestAddWorktreeRecordsCreateSpan(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "login", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	_, err := AddWorktree(ctx, r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// PostCreateSetup's own "copy" span also fires (CopyFiles defaults to
	// empty, but the copy phase always runs regardless of SkipDeps/SkipHooks),
	// so assert on the first span specifically rather than the total count.
	if len(sink.metrics) == 0 {
		t.Fatal("expected at least one span to be recorded")
	}
	span, ok := sink.metrics[0].(observability.SpanRecord)
	if !ok {
		t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
	}
	if span.Name != "create" {
		t.Errorf("span.Name = %q, want %q", span.Name, "create")
	}
}

func TestAddWorktreeBranchExists(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// BranchExists: rev-parse succeeds = branch exists
			if len(args) > 0 && args[0] == cmdRevParse {
				return "abc123", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			WorktreeDir: "/tmp/wt",
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddWorktreePathExists(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")
	// Create the worktree path so it already exists
	_ = os.MkdirAll(filepath.Join(wtDir, "feature-login"), 0o755)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error for existing path")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestAddWorktreeCreateFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree {
				return "", errors.New("cannot create worktree")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			WorktreeDir: "/tmp/nonexistent-wt",
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot create worktree") {
		t.Errorf("expected create error, got: %v", err)
	}
}

func TestAddWorktreeProgressCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	var messages []string
	onProgress := progress.Func(func(msg string) { messages = append(messages, msg) })

	_, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, onProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 progress messages, got %d: %v", len(messages), messages)
	}
}

func TestAddWorktreeWithDeps(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			// ListWorktrees for deps
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == cmdList {
				return "worktree " + tmpDir + "\nHEAD abc\nbranch refs/heads/main\n\n", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    false,
			AutoDetect:  false,
			SkipHooks:   true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DepsResults will be nil because no package.json etc. exists in the tmpDir
	if result.DepsResults != nil {
		t.Errorf("expected nil deps results (no modules), got %v", result.DepsResults)
	}
}

func TestAddWorktreeRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name    string
		task    string
		service string
	}{
		{name: "leading dash task", task: "-oops"},
		{name: "dotdot task", task: "foo..bar"},
		{name: "space in task", task: "a b"},
		{name: "semicolon in task", task: "a;b"},
		{name: "unsafe service", task: "x", service: ".."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &mockRunner{
				run: func(args ...string) (string, error) {
					t.Fatalf("git command should not run before validation, got args: %v", args)
					return "", nil
				},
				runInDir: func(dir string, args ...string) (string, error) {
					t.Fatalf("git command should not run before validation, got dir: %s args: %v", dir, args)
					return "", nil
				},
			}

			_, err := AddWorktree(context.Background(), r, AddParams{
				Task:    tc.task,
				Service: tc.service,
				Prefix:  "feature/",
				Source:  branchMain,
				PostCreateOptions: PostCreateOptions{
					WorktreeDir: "/tmp/wt",
					SkipDeps:    true,
					SkipHooks:   true,
				},
			}, nil)
			if err == nil {
				t.Fatal("expected error for unsafe input")
			}
			if !errors.Is(err, gitref.ErrUnsafeRefName) {
				t.Errorf("expected errors.Is ErrUnsafeRefName, got: %v", err)
			}
		})
	}
}

func TestAddWorktreeRejectsEmptyTask(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			t.Fatalf("git command should not run before validation, got args: %v", args)
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			t.Fatalf("git command should not run before validation, got dir: %s args: %v", dir, args)
			return "", nil
		},
	}

	_, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			WorktreeDir: "/tmp/wt",
			SkipDeps:    true,
			SkipHooks:   true,
		},
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty task")
	}
	if !strings.Contains(err.Error(), "task name is required") {
		t.Errorf("expected 'task name is required' error, got: %v", err)
	}
}

func TestAddWorktreeAllowsSafeInput(t *testing.T) {
	tests := []struct {
		name string
		task string
	}{
		{name: "simple task", task: "my-task"},
		{name: "pr review task", task: "review/123-slug"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			wtDir := filepath.Join(tmpDir, ".worktrees")

			r := &mockRunner{
				run: func(args ...string) (string, error) {
					if len(args) > 0 && args[0] == cmdRevParse {
						return "", errors.New("not found")
					}
					if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
						_ = os.MkdirAll(args[2], 0o755)
						return "", nil
					}
					return "", nil
				},
				runInDir: noopRunInDir,
			}

			_, err := AddWorktree(context.Background(), r, AddParams{
				Task:   tc.task,
				Prefix: "feature/",
				Source: branchMain,
				PostCreateOptions: PostCreateOptions{
					RepoRoot:    tmpDir,
					WorktreeDir: wtDir,
					SkipDeps:    true,
					SkipHooks:   true,
				},
			}, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAddWorktreeWithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			if len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd {
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := AddWorktree(context.Background(), r, AddParams{
		Task:   "login",
		Prefix: "feature/",
		Source: branchMain,
		PostCreateOptions: PostCreateOptions{
			RepoRoot:    tmpDir,
			WorktreeDir: wtDir,
			SkipDeps:    true,
			SkipHooks:   false,
			PostCreate:  [][]string{{"echo hello"}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.HookResults) != 1 {
		t.Fatalf("expected 1 hook result, got %d", len(result.HookResults))
	}
}
