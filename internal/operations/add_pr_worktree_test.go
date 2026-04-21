package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/progress"
)

const (
	ghArgAuth       = "auth"
	gitCmdFetch     = "fetch"
	gitCmdRemote    = "remote"
	gitSubcmdGetURL = "get-url"

	sameRepoPRJSON = `{
  "number": 42,
  "title": "Fix login redirect",
  "headRefName": "fix-login-redirect",
  "headRepository": {"name": "rimba"},
  "headRepositoryOwner": {"login": "lugassawan"},
  "isCrossRepository": false
}`

	crossForkPRJSON = `{
  "number": 99,
  "title": "Add OAuth support",
  "headRefName": "feat-oauth",
  "headRepository": {"name": "rimba"},
  "headRepositoryOwner": {"login": "contributor"},
  "isCrossRepository": true
}`
)

// mockGhRunner implements gh.Runner for testing.
type mockGhRunner struct {
	run func(ctx context.Context, args ...string) ([]byte, error)
}

func (m *mockGhRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	return m.run(ctx, args...)
}

func newGhAuthOK(prJSON string) *mockGhRunner {
	return &mockGhRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == ghArgAuth {
				return []byte("Logged in"), nil
			}
			return []byte(prJSON), nil
		},
	}
}

// makePRGitRunner returns a mockRunner for the happy-path PR add.
// It simulates: fetch succeeds, BranchExists=false, worktree add creates dir, remote helpers.
func makePRGitRunner(_ string, crossFork bool) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) > 0 && args[0] == gitCmdFetch:
				return "", nil
			case len(args) > 0 && args[0] == gitCmdRemote:
				if args[1] == gitSubcmdGetURL {
					if crossFork {
						return "", errors.New("no such remote") // remote doesn't exist yet
					}
					return "https://github.com/owner/repo.git", nil
				}
				if args[1] == gitSubcmdAdd {
					return "", nil
				}
			case len(args) > 0 && args[0] == cmdRevParse:
				return "", errors.New("not found") // branch doesn't exist
			case len(args) > 0 && args[0] == gitCmdWorktree && len(args) > 1 && args[1] == gitSubcmdAdd:
				_ = os.MkdirAll(args[2], 0o755)
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

func TestAddPRWorktreeSameRepo(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(sameRepoPRJSON)
	gitR := makePRGitRunner(tmpDir, false)

	result, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:    42,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err != nil {
		t.Fatalf("AddPRWorktree: %v", err)
	}
	if result.Branch != "review/42-fix-login-redirect" {
		t.Errorf("Branch = %q, want %q", result.Branch, "review/42-fix-login-redirect")
	}
	if result.Source != "origin/fix-login-redirect" {
		t.Errorf("Source = %q, want %q", result.Source, "origin/fix-login-redirect")
	}
}

func TestAddPRWorktreeCrossFork(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(crossForkPRJSON)
	gitR := makePRGitRunner(tmpDir, true)

	result, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:    99,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err != nil {
		t.Fatalf("AddPRWorktree: %v", err)
	}
	if result.Branch != "review/99-add-oauth-support" {
		t.Errorf("Branch = %q, want %q", result.Branch, "review/99-add-oauth-support")
	}
	if result.Source != "gh-fork-contributor/feat-oauth" {
		t.Errorf("Source = %q, want %q", result.Source, "gh-fork-contributor/feat-oauth")
	}
}

func TestAddPRWorktreeTaskOverride(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(sameRepoPRJSON)
	gitR := makePRGitRunner(tmpDir, false)

	result, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:     42,
		TaskOverride: "my-review",
		RepoRoot:     tmpDir,
		WorktreeDir:  wtDir,
		SkipDeps:     true,
		SkipHooks:    true,
	}, nil)
	if err != nil {
		t.Fatalf("AddPRWorktree: %v", err)
	}
	if result.Branch != "my-review" {
		t.Errorf("Branch = %q, want %q", result.Branch, "my-review")
	}
}

func TestAddPRWorktreeAuthFailure(t *testing.T) {
	ghR := &mockGhRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == ghArgAuth {
				return nil, errors.New("not authenticated")
			}
			return nil, nil
		},
	}
	gitR := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	_, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber: 42,
	}, nil)
	if err == nil {
		t.Fatal("expected error from auth failure")
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Errorf("expected auth hint, got: %v", err)
	}
}

func TestAddPRWorktreeFetchPRMetaFailure(t *testing.T) {
	ghR := &mockGhRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == ghArgAuth {
				return []byte("Logged in"), nil
			}
			return nil, errors.New("PR not found")
		},
	}
	gitR := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	_, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{PRNumber: 999}, nil)
	if err == nil {
		t.Fatal("expected error from FetchPRMeta failure")
	}
	if !strings.Contains(err.Error(), "verify PR number") {
		t.Errorf("expected 'verify PR number' hint, got: %v", err)
	}
}

func TestAddPRWorktreeSameRepoFetchFails(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(sameRepoPRJSON)
	gitR := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdFetch {
				return "", errors.New("network unreachable")
			}
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:    42,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected error from fetch failure")
	}
	if !strings.Contains(err.Error(), "network connectivity") {
		t.Errorf("expected 'network connectivity' hint, got: %v", err)
	}
}

func TestAddPRWorktreeCrossForkAddRemoteFails(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(crossForkPRJSON)
	gitR := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdRemote {
				if args[1] == gitSubcmdGetURL {
					return "", errors.New("no such remote")
				}
				if args[1] == gitSubcmdAdd {
					return "", errors.New("remote add failed")
				}
			}
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:    99,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected error from AddRemote failure")
	}
	if !strings.Contains(err.Error(), "fork visibility") {
		t.Errorf("expected 'fork visibility' hint, got: %v", err)
	}
}

func TestAddPRWorktreeCrossForkFetchFails(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(crossForkPRJSON)
	gitR := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdRemote {
				if args[1] == gitSubcmdGetURL {
					return "", errors.New("no such remote")
				}
				if args[1] == gitSubcmdAdd {
					return "", nil
				}
			}
			if len(args) > 0 && args[0] == gitCmdFetch {
				return "", errors.New("network unreachable")
			}
			if len(args) > 0 && args[0] == cmdRevParse {
				return "", errors.New("not found")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:    99,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected error from fork fetch failure")
	}
	if !strings.Contains(err.Error(), "fork visibility") {
		t.Errorf("expected 'fork visibility' hint, got: %v", err)
	}
}

func TestAddPRWorktreeResolveSourceFailure(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(sameRepoPRJSON)
	gitR := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitCmdFetch {
				return "", errors.New("fetch error")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	_, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:    42,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, nil)
	if err == nil {
		t.Fatal("expected error from resolveSource failure")
	}
}

func TestAddPRWorktreeProgressCallbacks(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, ".worktrees")

	ghR := newGhAuthOK(sameRepoPRJSON)
	gitR := makePRGitRunner(tmpDir, false)

	var messages []string
	onProgress := progress.Func(func(msg string) { messages = append(messages, msg) })

	_, err := AddPRWorktree(context.Background(), gitR, ghR, AddPRParams{
		PRNumber:    42,
		RepoRoot:    tmpDir,
		WorktreeDir: wtDir,
		SkipDeps:    true,
		SkipHooks:   true,
	}, onProgress)
	if err != nil {
		t.Fatalf("AddPRWorktree: %v", err)
	}
	if len(messages) == 0 {
		t.Error("expected progress messages, got none")
	}
}
