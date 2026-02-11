package operations

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

// mockRunner implements git.Runner for testing.
type mockRunner struct {
	runFn      func(args ...string) (string, error)
	runInDirFn func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	if m.runFn != nil {
		return m.runFn(args...)
	}
	return "", nil
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	if m.runInDirFn != nil {
		return m.runInDirFn(dir, args...)
	}
	return "", nil
}

const (
	testRepoRoot        = "/repo"
	cmdRevParseToplevel = "rev-parse --show-toplevel"
	cmdRevParseVerify   = "rev-parse --verify"
	cmdWorktreeList     = "worktree list"
	testTask            = "my-task"
	testFeaturePrefix   = "feature/"
	testFeatureBranch   = "feature/my-task"
	testWorktreeDir     = "../worktrees"
	testWtFeatureTask   = "/wt/feature/task"
	testBranchTask      = "feature/task"
	testWtFeatureSrc    = "/wt/feature/src"
	testBranchSrc       = "feature/src"
	testBranchDst       = "feature/dst"
	testBranchA         = "feature/a"
	fmtRemoveWorktree   = "RemoveWorktree: %v"
	errMsgUncommitted   = "uncommitted changes"
	fmtErrUncommitted   = "error = %q, want it to contain 'uncommitted changes'"
)

var errGit = errors.New("git error")

type wtEntry struct {
	path, branch string
}

func porcelain(entries ...wtEntry) string {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString("worktree " + e.path + "\n")
		b.WriteString("HEAD 0000000000000000000000000000000000000000\n")
		b.WriteString("branch refs/heads/" + e.branch + "\n")
		b.WriteString("\n")
	}
	return b.String()
}

func TestAddWorktreeSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return tmpDir, nil
			}
			if strings.HasPrefix(joined, cmdRevParseVerify) {
				return "", errors.New("branch not found")
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   "worktrees",
		DefaultSource: "main",
	}

	result, err := AddWorktree(r, cfg, testTask, testFeaturePrefix, "")
	if err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if result.Task != testTask {
		t.Errorf("Task = %q, want %q", result.Task, testTask)
	}
	if result.Branch != testFeatureBranch {
		t.Errorf("Branch = %q, want %q", result.Branch, testFeatureBranch)
	}
	if result.Source != "main" {
		t.Errorf("Source = %q, want %q", result.Source, "main")
	}
}

func TestAddWorktreeCustomSource(t *testing.T) {
	tmpDir := t.TempDir()
	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return tmpDir, nil
			}
			if strings.HasPrefix(joined, cmdRevParseVerify) {
				return "", errors.New("branch not found")
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   "worktrees",
		DefaultSource: "main",
	}

	result, err := AddWorktree(r, cfg, testTask, testFeaturePrefix, "develop")
	if err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if result.Source != "develop" {
		t.Errorf("Source = %q, want %q", result.Source, "develop")
	}
}

func TestAddWorktreeBranchExists(t *testing.T) {
	tmpDir := t.TempDir()
	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return tmpDir, nil
			}
			if strings.HasPrefix(joined, cmdRevParseVerify) {
				return "abc123", nil
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   "worktrees",
		DefaultSource: "main",
	}

	_, err := AddWorktree(r, cfg, "existing", testFeaturePrefix, "")
	if err == nil {
		t.Fatal("expected error when branch already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to contain 'already exists'", err)
	}
}

func TestAddWorktreeRepoRootError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errGit
		},
	}

	cfg := &config.Config{
		WorktreeDir:   "worktrees",
		DefaultSource: "main",
	}

	_, err := AddWorktree(r, cfg, "task", testFeaturePrefix, "")
	if err == nil {
		t.Fatal("expected error when RepoRoot fails")
	}
}

func TestRemoveWorktreeSuccess(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{"/wt/feature/my-task", testFeatureBranch},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
	}

	result, err := RemoveWorktree(r, testTask, false, false)
	if err != nil {
		t.Fatalf(fmtRemoveWorktree, err)
	}
	if result.Branch != testFeatureBranch {
		t.Errorf("Branch = %q, want %q", result.Branch, testFeatureBranch)
	}
	if !result.BranchDeleted {
		t.Error("expected branch to be deleted")
	}
}

func TestRemoveWorktreeKeepBranch(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureTask, testBranchTask},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
	}

	result, err := RemoveWorktree(r, "task", false, true)
	if err != nil {
		t.Fatalf(fmtRemoveWorktree, err)
	}
	if result.BranchDeleted {
		t.Error("expected branch NOT to be deleted when keepBranch=true")
	}
}

func TestRemoveWorktreeNotFound(t *testing.T) {
	worktreeOutput := porcelain(wtEntry{testRepoRoot, "main"})

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
	}

	_, err := RemoveWorktree(r, "nonexistent", false, false)
	if err == nil {
		t.Fatal("expected error when worktree not found")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("error = %q, want it to contain 'worktree not found'", err)
	}
}

func TestRemoveWorktreeBranchDeleteError(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureTask, testBranchTask},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			if strings.HasPrefix(joined, "branch") {
				return "", errors.New("branch delete failed")
			}
			return "", nil
		},
	}

	result, err := RemoveWorktree(r, "task", false, false)
	if err != nil {
		t.Fatalf(fmtRemoveWorktree, err)
	}
	if result.BranchDeleted {
		t.Error("expected branch NOT deleted on error")
	}
	if result.BranchError == nil {
		t.Error("expected BranchError to be set")
	}
}

func TestMergeWorktreeSuccess(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureSrc, testBranchSrc},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	result, err := MergeWorktree(r, cfg, "src", "", false, false)
	if err != nil {
		t.Fatalf("MergeWorktree: %v", err)
	}
	if result.SourceBranch != testBranchSrc {
		t.Errorf("SourceBranch = %q, want %q", result.SourceBranch, testBranchSrc)
	}
	if result.TargetLabel != "main" {
		t.Errorf("TargetLabel = %q, want %q", result.TargetLabel, "main")
	}
}

func TestMergeWorktreeIntoAnother(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureSrc, testBranchSrc},
		wtEntry{"/wt/feature/dst", testBranchDst},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	result, err := MergeWorktree(r, cfg, "src", "dst", false, false)
	if err != nil {
		t.Fatalf("MergeWorktree: %v", err)
	}
	if result.TargetLabel != testBranchDst {
		t.Errorf("TargetLabel = %q, want %q", result.TargetLabel, testBranchDst)
	}
}

func TestMergeWorktreeSourceNotFound(t *testing.T) {
	worktreeOutput := porcelain(wtEntry{testRepoRoot, "main"})

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	_, err := MergeWorktree(r, cfg, "nonexistent", "", false, false)
	if err == nil {
		t.Fatal("expected error when source not found")
	}
}

func TestMergeWorktreeTargetNotFound(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureSrc, testBranchSrc},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	_, err := MergeWorktree(r, cfg, "src", "nonexistent", false, false)
	if err == nil {
		t.Fatal("expected error when target not found")
	}
}

func TestMergeWorktreeDirtySource(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureSrc, testBranchSrc},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(dir string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "status") && strings.Contains(dir, "src") {
				return "M file.go", nil
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	_, err := MergeWorktree(r, cfg, "src", "", false, false)
	if err == nil {
		t.Fatal("expected error for dirty source")
	}
	if !strings.Contains(err.Error(), errMsgUncommitted) {
		t.Errorf(fmtErrUncommitted, err)
	}
}

func TestMergeWorktreeDirtyTarget(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureSrc, testBranchSrc},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(dir string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "status") && dir == testRepoRoot {
				return "M file.go", nil
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	_, err := MergeWorktree(r, cfg, "src", "", false, false)
	if err == nil {
		t.Fatal("expected error for dirty target")
	}
	if !strings.Contains(err.Error(), errMsgUncommitted) {
		t.Errorf(fmtErrUncommitted, err)
	}
}

func TestSyncWorktreeRebase(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureTask, testBranchTask},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}

	if err := SyncWorktree(r, "task", "main", false); err != nil {
		t.Fatalf("SyncWorktree: %v", err)
	}
}

func TestSyncWorktreeMerge(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureTask, testBranchTask},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}

	if err := SyncWorktree(r, "task", "main", true); err != nil {
		t.Fatalf("SyncWorktree with merge: %v", err)
	}
}

func TestSyncWorktreeNotFound(t *testing.T) {
	worktreeOutput := porcelain(wtEntry{testRepoRoot, "main"})

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
	}

	err := SyncWorktree(r, "nonexistent", "main", false)
	if err == nil {
		t.Fatal("expected error for nonexistent worktree")
	}
}

func TestSyncWorktreeDirty(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureTask, testBranchTask},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "status") {
				return "M dirty.go", nil
			}
			return "", nil
		},
	}

	err := SyncWorktree(r, "task", "main", false)
	if err == nil {
		t.Fatal("expected error for dirty worktree")
	}
	if !strings.Contains(err.Error(), errMsgUncommitted) {
		t.Errorf(fmtErrUncommitted, err)
	}
}

func TestSyncWorktreeRebaseFailure(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureTask, testBranchTask},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "status") {
				return "", nil
			}
			if strings.HasPrefix(joined, "rebase") && !strings.Contains(joined, "--abort") {
				return "", errors.New("rebase conflict")
			}
			return "", nil
		},
	}

	err := SyncWorktree(r, "task", "main", false)
	if err == nil {
		t.Fatal("expected error for rebase failure")
	}
}

func TestListWorktreeInfos(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{"/wt/feature/a", testBranchA},
		wtEntry{"/wt/bugfix/b", "bugfix/b"},
	)

	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return worktreeOutput, nil
		},
	}

	infos, err := listWorktreeInfos(r)
	if err != nil {
		t.Fatalf("listWorktreeInfos: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("got %d infos, want 3", len(infos))
	}
	if infos[1].Branch != testBranchA {
		t.Errorf("infos[1].Branch = %q, want %q", infos[1].Branch, testBranchA)
	}
}

func TestListWorktreeInfosError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errGit
		},
	}

	_, err := listWorktreeInfos(r)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMergeWorktreeWithDelete(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureSrc, testBranchSrc},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	result, err := MergeWorktree(r, cfg, "src", "", false, true)
	if err != nil {
		t.Fatalf("MergeWorktree with delete: %v", err)
	}
	if !result.Deleted {
		t.Error("expected Deleted to be true")
	}
}

func TestMergeWorktreeRepoRootError(t *testing.T) {
	r := &mockRunner{
		runFn: func(_ ...string) (string, error) {
			return "", errGit
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	_, err := MergeWorktree(r, cfg, "src", "", false, false)
	if err == nil {
		t.Fatal("expected error when RepoRoot fails")
	}
}

func TestMergeWorktreeMergeFails(t *testing.T) {
	worktreeOutput := porcelain(
		wtEntry{testRepoRoot, "main"},
		wtEntry{testWtFeatureSrc, testBranchSrc},
	)

	r := &mockRunner{
		runFn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, cmdRevParseToplevel) {
				return testRepoRoot, nil
			}
			if strings.HasPrefix(joined, cmdWorktreeList) {
				return worktreeOutput, nil
			}
			return "", nil
		},
		runInDirFn: func(_ string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "status") {
				return "", nil
			}
			if strings.HasPrefix(joined, "merge") {
				return "", errors.New("merge conflict")
			}
			return "", nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   testWorktreeDir,
		DefaultSource: "main",
	}

	_, err := MergeWorktree(r, cfg, "src", "", false, false)
	if err == nil {
		t.Fatal("expected error when merge fails")
	}
	if !strings.Contains(err.Error(), "merge failed") {
		t.Errorf("error = %q, want it to contain 'merge failed'", err)
	}
}
