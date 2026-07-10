package operations

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	gitCmdStatus       = "status"
	gitCmdMerge        = "merge"
	gitCmdAbort        = "--abort"
	branchFeatureLogin = "feature/login"
	statusDirtyOutput  = "M dirty.go"
)

func mergeWorktreeList() string {
	return porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", branchFeatureLogin},
		struct{ path, branch string }{"/wt/feature-dashboard", "feature/dashboard"},
	)
}

func mergeRunner(mergeErr error) *mockRunner {
	wt := mergeWorktreeList()
	mergeInProgress := mergeErr != nil // MERGE_HEAD exists iff a merge failure is expected
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				if len(args) >= 2 && args[1] == gitCmdAbort {
					return "", nil // abort succeeds by default
				}
				return "", mergeErr
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				if !mergeInProgress {
					return "", errors.New("no MERGE_HEAD")
				}
				return "abc1234", nil // MERGE_HEAD exists
			}
			return "", nil
		},
	}
}

func TestMergeWorktreeMergeToMain(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceBranch != branchFeatureLogin {
		t.Errorf("expected source branch 'feature/login', got %q", result.SourceBranch)
	}
	if result.TargetLabel != "main" {
		t.Errorf("expected target label 'main', got %q", result.TargetLabel)
	}
	if !result.MergingToMain {
		t.Error("expected MergingToMain to be true")
	}
	if !result.SourceRemoved {
		t.Error("expected source to be auto-removed when merging to main")
	}
}

func TestMergeWorktreePreResolvedSourceSkipsWorktreeList(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				t.Fatal("MergeWorktree should not list worktrees when Source is pre-resolved and merging to main")
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errors.New("no MERGE_HEAD")
			}
			return "", nil
		},
	}

	source := &resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: branchFeatureLogin}
	result, err := MergeWorktree(context.Background(), r, MergeParams{
		Source:     source,
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceBranch != branchFeatureLogin {
		t.Errorf("SourceBranch = %q, want %q", result.SourceBranch, branchFeatureLogin)
	}
	if result.SourcePath != "/wt/feature-login" {
		t.Errorf("SourcePath = %q, want %q", result.SourcePath, "/wt/feature-login")
	}
}

func TestMergeWorktreeCustomPrefix(t *testing.T) {
	porcelain := porcelainEntries(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/PROJ-123", branchProj123},
	)
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errors.New("no MERGE_HEAD")
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(customPrefixContext(), r, MergeParams{
		SourceTask: "123",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("MergeWorktree with custom prefix: %v", err)
	}
	if result.SourceBranch != branchProj123 {
		t.Errorf("SourceBranch = %q, want %q", result.SourceBranch, branchProj123)
	}

	// No config in context: built-ins-only PrefixSet cannot resolve task "123"
	// against branch branchProj123.
	_, err = MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "123",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected MergeWorktree to fail without custom prefix config (built-ins-only parity)")
	}
}

func TestMergeWorktreeMergeToMainKeep(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceRemoved {
		t.Error("expected source NOT to be removed when keep=true")
	}
}

func TestMergeWorktreeMergeToWorktree(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		IntoTask:   "dashboard",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceBranch != branchFeatureLogin {
		t.Errorf("expected source 'feature/login', got %q", result.SourceBranch)
	}
	if result.TargetLabel != "feature/dashboard" {
		t.Errorf("expected target 'feature/dashboard', got %q", result.TargetLabel)
	}
	if result.MergingToMain {
		t.Error("expected MergingToMain to be false")
	}
	if result.SourceRemoved {
		t.Error("expected source NOT to be removed when merging to worktree without delete")
	}
}

func TestMergeWorktreeMergeToWorktreeWithDelete(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		IntoTask:   "dashboard",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Delete:     true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SourceRemoved {
		t.Error("expected source to be removed when delete=true")
	}
}

func TestMergeWorktreeSourceNotFound(t *testing.T) {
	r := mergeRunner(nil)

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "nonexistent",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestMergeWorktreeTargetNotFound(t *testing.T) {
	r := mergeRunner(nil)

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		IntoTask:   "nonexistent",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestMergeWorktreeSourceDirty(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return wt, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				if strings.Contains(dir, "login") {
					return statusDirtyOutput, nil
				}
				return "", nil
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for dirty source")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("expected 'uncommitted changes', got: %v", err)
	}
}

func TestMergeWorktreeTargetDirty(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return wt, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				if dir == "/repo" {
					return statusDirtyOutput, nil
				}
				return "", nil
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error for dirty target")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("expected 'uncommitted changes', got: %v", err)
	}
}

func TestMergeWorktreeMergeConflict(t *testing.T) {
	r := mergeRunner(errors.New("conflict"))

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true, // avoid cleanup attempt
	}, nil)
	if err == nil {
		t.Fatal("expected error from merge conflict")
	}
	if !strings.Contains(err.Error(), "merge failed") {
		t.Errorf("expected 'merge failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "restored to pre-merge state") {
		t.Errorf("expected 'restored to pre-merge state', got: %v", err)
	}
}

func TestMergeWorktreeCleanupPartialFailure(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == "list" {
				return wt, nil
			}
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == "remove" {
				return "", errors.New("worktree locked")
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("expected no fatal error (cleanup failure is non-fatal), got: %v", err)
	}
	if result.SourceRemoved {
		t.Error("expected source NOT removed when cleanup fails")
	}
	if result.RemoveError == nil {
		t.Error("expected RemoveError to be set")
	}
}

// TestMergeWorktreeCleanupPrunableSurfacesOnResult guards the merge-side hint
// fix: when the source worktree is prunable, MergeResult.SourcePrunable must
// be set so cmd/merge.go can print a prune-based recovery hint instead of the
// doomed `git worktree remove --force` command (#374).
func TestMergeWorktreeCleanupPrunableSurfacesOnResult(t *testing.T) {
	wt := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /wt/feature-login",
		"HEAD def456",
		"branch refs/heads/feature/login",
		"prunable gitdir file points to non-existent location",
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == "list" {
				return wt, nil
			}
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == "prune" {
				return "", errors.New("prune failed")
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && (args[0] == gitCmdStatus || args[0] == gitCmdMerge) {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("expected no fatal error (cleanup failure is non-fatal), got: %v", err)
	}
	if !result.SourcePrunable {
		t.Error("expected SourcePrunable to be true")
	}
	if result.RemoveError == nil {
		t.Error("expected RemoveError to be set when git worktree prune fails")
	}
}

func TestMergeWorktreeProgressCallbacks(t *testing.T) {
	r := mergeRunner(nil)

	var messages []string
	onProgress := progress.Func(func(msg string) { messages = append(messages, msg) })

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, onProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 progress messages, got %d: %v", len(messages), messages)
	}
}

func TestMergeWorktreeListWorktreesFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("worktree list failed")
		},
		runInDir: noopRunInDir,
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMergeWorktreeSourceDirtyCheckError(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			// Source worktree IsDirty check fails
			if dir == "/wt/feature-login" {
				return "", errors.New("git status failed")
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: branchMain,
	}, nil)
	if err == nil {
		t.Fatal("expected error from source dirty check")
	}
	if !strings.Contains(err.Error(), "git status failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMergeWorktreeDryRun(t *testing.T) {
	r := mergeRunner(nil)

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		DryRun:     true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SourceRemoved {
		t.Error("expected source NOT removed in dry-run mode")
	}
	if result.Plan == nil {
		t.Fatal("expected Plan to be non-nil in dry-run mode")
	}
	if len(result.Plan.Steps) == 0 {
		t.Error("expected at least one planned step")
	}
	hasMerge := false
	for _, s := range result.Plan.Steps {
		if strings.Contains(s, "merge") {
			hasMerge = true
			break
		}
	}
	if !hasMerge {
		t.Errorf("expected a merge step in plan, got: %v", result.Plan.Steps)
	}
}

func TestMergeWorktreeTargetDirtyCheckError(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			// Target (repo root) IsDirty check fails
			if dir == "/repo" {
				return "", errors.New("target status failed")
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: branchMain,
	}, nil)
	if err == nil {
		t.Fatal("expected error from target dirty check")
	}
	if !strings.Contains(err.Error(), "target status failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMergeWorktreeMergeFailsAbortAlsoFails(t *testing.T) {
	abortErr := errors.New("abort failed")
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				if len(args) >= 2 && args[1] == gitCmdAbort {
					return "", abortErr
				}
				return "", errors.New("conflict")
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "abc1234", nil // MERGE_HEAD exists
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rollback failed") {
		t.Errorf("expected 'rollback failed', got: %v", err)
	}
}

func TestMergeWorktreeMergeFailsNoMergeInProgress(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", errors.New("conflict")
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errors.New("no MERGE_HEAD") // merge never started
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "merge failed") {
		t.Errorf("expected 'merge failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "target") || !strings.Contains(err.Error(), "unchanged") {
		t.Errorf("expected 'target ... unchanged', got: %v", err)
	}
}

// mergeCleanupBranchDeleteFailsRunner returns a runner where merge + worktree
// removal succeed but branch deletion fails. Extracted to keep
// TestMergeWorktreeCleanupBranchDeleteFails under the gocyclo limit.
func mergeCleanupBranchDeleteFailsRunner(wt string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == gitSubcmdList {
				return wt, nil
			}
			if len(args) >= 2 && args[0] == gitCmdWorktree && args[1] == "remove" {
				return "", nil
			}
			if len(args) >= 2 && args[0] == cmdBranch && args[1] == "-D" {
				return "", errors.New("branch in use")
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}
}

func TestMergeWorktreeCleanupBranchDeleteFails(t *testing.T) {
	r := mergeCleanupBranchDeleteFailsRunner(mergeWorktreeList())

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("expected no fatal error (branch delete failure is non-fatal), got: %v", err)
	}
	if result.SourceRemoved {
		t.Error("expected SourceRemoved=false when branch delete fails")
	}
	if result.RemoveError == nil {
		t.Fatal("expected RemoveError to be set")
	}
	if !strings.Contains(result.RemoveError.Error(), "failed to delete branch") {
		t.Errorf("expected unified hint in RemoveError, got: %v", result.RemoveError)
	}
	if !strings.Contains(result.RemoveError.Error(), "git branch -D "+branchFeatureLogin) {
		t.Errorf("expected branch name in RemoveError hint, got: %v", result.RemoveError)
	}
}

func TestMergeWorktreeRemoteDeleteSuccess(t *testing.T) {
	wt := mergeWorktreeList()
	var pushArgs []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) >= 1 && args[0] == gitCmdPush {
				pushArgs = args
				return "", nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.RemoteDeleted {
		t.Error("expected RemoteDeleted=true")
	}
	if result.RemoteError != nil {
		t.Errorf("expected nil RemoteError, got: %v", result.RemoteError)
	}
	if len(pushArgs) == 0 {
		t.Error("expected git push --delete to be called")
	}
	if !slices.Contains(pushArgs, "--delete") {
		t.Errorf("expected --delete flag in push args, got: %v", pushArgs)
	}
}

func TestMergeWorktreeRemoteDeleteFailureNonFatal(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) >= 1 && args[0] == gitCmdPush {
				return "", errors.New("connection refused")
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("expected no fatal error (remote delete is best-effort), got: %v", err)
	}
	if result.RemoteDeleted {
		t.Error("expected RemoteDeleted=false on failure")
	}
	if result.RemoteError == nil {
		t.Error("expected non-nil RemoteError")
	}
}

func TestMergeWorktreeNoOriginSkipsRemoteDelete(t *testing.T) {
	wt := mergeWorktreeList()
	pushCalled := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) >= 2 && args[0] == "remote" && args[1] == "get-url" {
				return "", errors.New("no such remote")
			}
			if len(args) >= 1 && args[0] == gitCmdPush {
				pushCalled = true
				return "", nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RemoteDeleted {
		t.Error("expected RemoteDeleted=false when no origin")
	}
	if pushCalled {
		t.Error("expected no push call when no origin")
	}
}

func TestMergeWorktreeKeepSuppressesRemoteDelete(t *testing.T) {
	wt := mergeWorktreeList()
	remoteChecked := false
	pushCalled := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) >= 1 && args[0] == "remote" {
				remoteChecked = true
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdPush {
				pushCalled = true
				return "", nil
			}
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		Keep:       true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RemoteDeleted {
		t.Error("expected RemoteDeleted=false with --keep")
	}
	if remoteChecked {
		t.Error("expected no remote check when --keep suppresses cleanup")
	}
	if pushCalled {
		t.Error("expected no push when --keep suppresses cleanup")
	}
}

func TestMergeWorktreeDryRunIncludesRemoteStep(t *testing.T) {
	wt := mergeWorktreeList()
	pushCalled := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitCmdWorktree {
				return wt, nil
			}
			if len(args) >= 1 && args[0] == gitCmdPush {
				pushCalled = true
				return "", nil
			}
			// remote get-url succeeds → origin present
			return "", nil
		},
		runInDir: func(_ string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdMerge {
				return "", nil
			}
			return "", nil
		},
	}

	result, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		RepoRoot:   "/repo",
		MainBranch: "main",
		DryRun:     true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasRemoteStep := false
	for _, s := range result.Plan.Steps {
		if strings.Contains(s, "delete remote branch") && strings.Contains(s, "origin/"+branchFeatureLogin) {
			hasRemoteStep = true
			break
		}
	}
	if !hasRemoteStep {
		t.Errorf("expected remote step in plan, got: %v", result.Plan.Steps)
	}
	if pushCalled {
		t.Error("expected no push in dry-run mode")
	}
}

func TestMergeWorktreeTargetDirtyToWorktree(t *testing.T) {
	wt := mergeWorktreeList()
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return wt, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == gitCmdStatus {
				if dir == "/wt/feature-dashboard" {
					return statusDirtyOutput, nil
				}
				return "", nil
			}
			return "", nil
		},
	}

	_, err := MergeWorktree(context.Background(), r, MergeParams{
		SourceTask: "login",
		IntoTask:   "dashboard",
		RepoRoot:   "/repo",
		MainBranch: branchMain,
	}, nil)
	if err == nil {
		t.Fatal("expected error for dirty target worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("expected 'uncommitted changes', got: %v", err)
	}
}
