package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/resolver"
)

func duplicateParams(repoRoot string) DuplicateParams {
	options := PostCreateOptions{
		RepoRoot:    repoRoot,
		WorktreeDir: filepath.Join(repoRoot, ".worktrees"),
		SkipDeps:    true,
		SkipHooks:   true,
	}
	return DuplicateParams{
		Source: resolver.WorktreeInfo{
			Path:   filepath.Join(repoRoot, "source"),
			Branch: "feature/task",
		},
		SourceTask:        "task",
		PrefixSet:         resolver.DefaultPrefixSet(),
		DefaultSource:     branchMain,
		PostCreateOptions: options,
	}
}

func TestDuplicateWorktreeAutoSuffix(t *testing.T) {
	repoRoot := t.TempDir()
	params := duplicateParams(repoRoot)
	for _, task := range []string{"task-2", "task-3"} {
		path := resolver.WorktreePath(params.WorktreeDir, "feature/"+task)
		if task == "task-2" {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Symlink(filepath.Join(repoRoot, "missing"), path); err != nil {
			t.Fatal(err)
		}
	}

	var createdBranch string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case args[0] == "rev-parse" && strings.HasSuffix(args[len(args)-1], "feature/task-1"):
				return "abc", nil
			case len(args) > 1 && args[0] == "worktree" && args[1] == "add":
				createdBranch = args[3]
				if err := os.MkdirAll(args[5], 0o755); err != nil {
					t.Fatal(err)
				}
				return "", nil
			}
			return "", errGitFailed
		},
		runInDir: noopRunInDir,
	}

	result, err := DuplicateWorktree(context.Background(), r, params, nil)
	if err != nil {
		t.Fatalf("DuplicateWorktree: %v", err)
	}
	if result.Task != "task-4" || result.Branch != "feature/task-4" {
		t.Errorf("result = %+v, want task-4 on feature/task-4", result)
	}
	if createdBranch != "feature/task-4" {
		t.Errorf("created branch = %q, want feature/task-4", createdBranch)
	}
}

func TestDuplicateWorktreeCustomTarget(t *testing.T) {
	tests := []struct {
		name    string
		as      string
		prepare func(t *testing.T, repoRoot string)
		want    string
		wantErr string
	}{
		{name: "same service", as: "copy", want: "feature/copy"},
		{
			name: "retarget existing service",
			as:   "payments/copy",
			prepare: func(t *testing.T, repoRoot string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(repoRoot, "payments"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "payments/feature/copy",
		},
		{name: "unknown service", as: "ghost/copy", wantErr: `service "ghost" not found`},
		{name: "unsafe ref", as: "-copy", wantErr: "invalid task name"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			if tc.prepare != nil {
				tc.prepare(t, repoRoot)
			}
			params := duplicateParams(repoRoot)
			params.As = tc.as
			created := false
			r := &mockRunner{
				run: func(args ...string) (string, error) {
					if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
						created = true
						if err := os.MkdirAll(args[5], 0o755); err != nil {
							t.Fatal(err)
						}
						return "", nil
					}
					return "", errGitFailed
				},
				runInDir: noopRunInDir,
			}

			result, err := DuplicateWorktree(context.Background(), r, params, nil)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				if created {
					t.Fatal("must not create a worktree for invalid target")
				}
				return
			}
			if err != nil {
				t.Fatalf("DuplicateWorktree: %v", err)
			}
			if result.Branch != tc.want {
				t.Errorf("branch = %q, want %q", result.Branch, tc.want)
			}
		})
	}
}

func TestDuplicateWorktreeRejectsDefaultBranch(t *testing.T) {
	params := duplicateParams(t.TempDir())
	params.Source.Branch = branchMain
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			t.Fatalf("unexpected git command: %v", args)
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := DuplicateWorktree(context.Background(), r, params, nil)
	if err == nil || !strings.Contains(err.Error(), "use 'rimba add' instead") {
		t.Fatalf("error = %v, want default-branch rejection", err)
	}
}

func TestDuplicateWorktreeDryRun(t *testing.T) {
	params := duplicateParams(t.TempDir())
	params.DryRun = true
	created := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
				created = true
			}
			return "", errGitFailed
		},
		runInDir: noopRunInDir,
	}

	result, err := DuplicateWorktree(context.Background(), r, params, nil)
	if err != nil {
		t.Fatalf("DuplicateWorktree: %v", err)
	}
	if !result.DryRun || result.Branch != "feature/task-1" {
		t.Errorf("result = %+v, want dry-run feature/task-1", result)
	}
	if created {
		t.Fatal("dry run must not create a worktree")
	}
}

func TestDuplicateWorktreeCreatesFromSource(t *testing.T) {
	repoRoot := t.TempDir()
	params := duplicateParams(repoRoot)
	var addArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
				addArgs = append([]string(nil), args...)
				if err := os.MkdirAll(args[5], 0o755); err != nil {
					t.Fatal(err)
				}
				return "", nil
			}
			return "", errGitFailed
		},
		runInDir: noopRunInDir,
	}

	result, err := DuplicateWorktree(context.Background(), r, params, nil)
	if err != nil {
		t.Fatalf("DuplicateWorktree: %v", err)
	}
	if got := addArgs[len(addArgs)-1]; got != params.Source.Branch {
		t.Errorf("source branch = %q, want %q", got, params.Source.Branch)
	}
	if result.SourceTask != "task" || result.SourceBranch != "feature/task" {
		t.Errorf("source result = %+v, want source task and branch", result)
	}
}

func TestDuplicateWorktreeRollsBackSetupFailure(t *testing.T) {
	params := duplicateParams(t.TempDir())
	params.SkipDeps = false
	removed, deleted := false, false

	_, err := DuplicateWorktree(context.Background(), duplicateRollbackRunner(t, &removed, &deleted), params, nil)
	if err == nil || !strings.Contains(err.Error(), "worktree list failed") {
		t.Fatalf("error = %v, want setup failure", err)
	}
	if !removed || !deleted {
		t.Errorf("rollback removed=%t deleted=%t, want both true", removed, deleted)
	}
}

func duplicateRollbackRunner(t *testing.T, removed, deleted *bool) *mockRunner {
	t.Helper()
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 1 && args[0] == "worktree" {
				switch args[1] {
				case "add":
					if err := os.MkdirAll(args[5], 0o755); err != nil {
						t.Fatal(err)
					}
					return "", nil
				case "list":
					return "", errors.New("worktree list failed")
				case "remove":
					*removed = true
					return "", nil
				}
			}
			if args[0] == "branch" {
				*deleted = true
				return "", nil
			}
			if args[0] == "rev-parse" {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

func TestDuplicateWorktreeRecordsCreateSpan(t *testing.T) {
	repoRoot := t.TempDir()
	params := duplicateParams(repoRoot)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
				if err := os.MkdirAll(args[5], 0o755); err != nil {
					t.Fatal(err)
				}
				return "", nil
			}
			return "", errGitFailed
		},
		runInDir: noopRunInDir,
	}
	sink := &fakeSink{}
	ctx := observability.WithRecorder(context.Background(), observability.Maybe(true, sink, "duplicate", "task", "", "test"))

	if _, err := DuplicateWorktree(ctx, r, params, nil); err != nil {
		t.Fatalf("DuplicateWorktree: %v", err)
	}
	if len(sink.metrics) == 0 {
		t.Fatal("expected create span")
	}
	span, ok := sink.metrics[0].(observability.SpanRecord)
	if !ok || span.Name != "create" {
		t.Errorf("first metric = %#v, want create span", sink.metrics[0])
	}
}

func TestNextDuplicateTask(t *testing.T) {
	tests := []struct {
		name     string
		service  string
		occupied []string
		branches []string
		want     string
	}{
		{name: "no collision", want: "login-1"},
		{name: "path collision", occupied: []string{"login-1"}, want: "login-2"},
		{name: "branch collision", branches: []string{"login-1"}, want: "login-2"},
		{name: "mixed collisions", occupied: []string{"login-1"}, branches: []string{"login-2"}, want: "login-3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			worktreeDir := t.TempDir()
			for _, task := range tc.occupied {
				path := resolver.WorktreePath(worktreeDir, resolver.FullBranchName(tc.service, "feature/", task))
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			r := &mockRunner{
				run: func(args ...string) (string, error) {
					for _, branch := range tc.branches {
						if strings.HasSuffix(args[len(args)-1], "/"+branch) {
							return "", nil
						}
					}
					return "", errGitFailed
				},
				runInDir: noopRunInDir,
			}
			got, err := nextDuplicateTask(context.Background(), r, "login", tc.service, "feature/", worktreeDir)
			if err != nil {
				t.Fatalf("nextDuplicateTask: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNextDuplicateTaskExhausted(t *testing.T) {
	r := &mockRunner{run: func(args ...string) (string, error) { return "", nil }, runInDir: noopRunInDir}
	_, err := nextDuplicateTask(context.Background(), r, "login", "", "feature/", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "could not find available suffix") {
		t.Fatalf("error = %v, want suffix exhaustion", err)
	}
}
