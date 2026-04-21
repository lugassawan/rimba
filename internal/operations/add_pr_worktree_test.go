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
	ghArgAuth = "auth"

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
			case len(args) > 0 && args[0] == "fetch":
				return "", nil
			case len(args) > 0 && args[0] == "remote":
				if args[1] == "get-url" {
					if crossFork {
						return "", errors.New("no such remote") // remote doesn't exist yet
					}
					return "https://github.com/owner/repo.git", nil
				}
				if args[1] == "add" {
					return "", nil
				}
			case len(args) > 0 && args[0] == "rev-parse":
				return "", errors.New("not found") // branch doesn't exist
			case len(args) > 0 && args[0] == "worktree" && len(args) > 1 && args[1] == "add":
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
