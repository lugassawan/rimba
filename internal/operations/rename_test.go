package operations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	cmdRevParse     = "rev-parse"
	cmdWorktree     = "worktree"
	cmdMove         = "move"
	wtDir           = "/worktrees"
	branchAuth      = "feature/auth"
	errMoveFailed   = "move failed"
	errRenameFailed = "rename failed"
	remoteURLStub   = "https://example.com/repo.git"
)

var errNoSuchRemote = errors.New("no such remote")

func TestRenameWorktreeSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if res.OldBranch != branchFeature {
		t.Errorf("OldBranch = %q, want %q", res.OldBranch, branchFeature)
	}
	if res.NewBranch != branchAuth {
		t.Errorf("NewBranch = %q, want %q", res.NewBranch, branchAuth)
	}
	if res.OldPath != pathWtFeatureLogin {
		t.Errorf("OldPath = %q, want %q", res.OldPath, pathWtFeatureLogin)
	}
}

func TestRenameWorktreeCustomPrefix(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchProj123, Path: "/wt/PROJ-123"}
	res, err := RenameWorktree(customPrefixContext(), r, RenameParams{WT: wt, NewTask: "456", WtDir: wtDir})
	if err != nil {
		t.Fatalf("RenameWorktree with custom prefix: %v", err)
	}
	if res.NewBranch != "PROJ-456" {
		t.Errorf("NewBranch = %q, want %q", res.NewBranch, "PROJ-456")
	}

	// No config in context: built-ins-only PrefixSet doesn't recognize "PROJ-",
	// so the rename falls back to the default "feature/" type.
	res, err = RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "456", WtDir: wtDir})
	if err != nil {
		t.Fatalf("RenameWorktree without custom prefix: %v", err)
	}
	if res.NewBranch != "feature/456" {
		t.Errorf("NewBranch = %q, want %q (default fallback)", res.NewBranch, "feature/456")
	}
}

func TestRenameWorktreeBranchExists(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", nil // BranchExists returns true
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir})
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git branch -D") {
		t.Errorf("error = %q, want 'git branch -D' hint fragment", err.Error())
	}
}

func TestRenameWorktreeSameName(t *testing.T) {
	calls := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			calls++
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			calls++
			return "", nil
		},
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "login", WtDir: wtDir})
	if err == nil {
		t.Fatal("expected error for same-name rename")
	}
	if !strings.Contains(err.Error(), "nothing to change") {
		t.Errorf("error = %q, want 'nothing to change' message", err.Error())
	}
	if strings.Contains(err.Error(), "git branch -D") {
		t.Errorf("error = %q, must not include destructive branch delete hint", err.Error())
	}
	if strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, must not include a recovery hint", err.Error())
	}
	if calls != 0 {
		t.Fatalf("expected no git calls for same-name rename, got %d", calls)
	}
}

func TestRenameWorktreeMoveFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktree && args[1] == cmdMove {
				return "", errors.New(errMoveFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir})
	if err == nil {
		t.Fatal("expected error from move failure")
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git worktree unlock") {
		t.Errorf("error = %q, want 'git worktree unlock' hint fragment", err.Error())
	}
}

func TestRenameWorktreeBranchRenameFails(t *testing.T) {
	moveCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktree && args[1] == cmdMove {
				moveCount++
				return "", nil
			}
			if len(args) >= 2 && args[0] == "branch" && args[1] == "-m" {
				return "", errors.New(errRenameFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir})
	if err == nil {
		t.Fatal("expected error from branch rename failure")
	}
	if !strings.Contains(err.Error(), "failed to rename branch") {
		t.Errorf("error = %q, want 'failed to rename branch'", err.Error())
	}
	if !strings.Contains(err.Error(), "moved back") {
		t.Errorf("error = %q, want 'moved back' (rollback confirmation)", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git branch -m") {
		t.Errorf("error = %q, want 'git branch -m' hint fragment", err.Error())
	}
	if moveCount != 2 {
		t.Errorf("expected 2 worktree move calls (forward + rollback), got %d", moveCount)
	}
}

func TestRenameWorktreeBranchRenameFailsRollbackFails(t *testing.T) {
	moveCount := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktree && args[1] == cmdMove {
				moveCount++
				if moveCount == 2 {
					return "", errors.New("rollback move failed")
				}
				return "", nil
			}
			if len(args) >= 2 && args[0] == "branch" && args[1] == "-m" {
				return "", errors.New(errRenameFailed)
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	_, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir})
	if err == nil {
		t.Fatal("expected error from branch rename + rollback failure")
	}
	if !strings.Contains(err.Error(), "failed to rename branch") {
		t.Errorf("error = %q, want 'failed to rename branch'", err.Error())
	}
	if !strings.Contains(err.Error(), "Rollback failed") {
		t.Errorf("error = %q, want 'Rollback failed'", err.Error())
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want 'To fix:' hint", err.Error())
	}
	if !strings.Contains(err.Error(), "git worktree move") {
		t.Errorf("error = %q, want 'git worktree move' hint fragment", err.Error())
	}
}

func TestRenameWorktreeNoPrefixMatch(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Branch: "plain-branch", Path: "/wt/plain-branch"}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "new-task", WtDir: wtDir})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if !strings.HasPrefix(res.NewBranch, "feature/") {
		t.Errorf("NewBranch = %q, want feature/ prefix", res.NewBranch)
	}
}

func TestRenameWorktreeTypeOnly(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	// feature/auth → bugfix/auth (same task, different prefix)
	wt := resolver.WorktreeInfo{Branch: branchAuth, Path: "/wt/feature-auth"}
	res, err := RenameWorktree(context.Background(), r, RenameParams{
		WT:        wt,
		NewTask:   "auth",
		NewPrefix: "bugfix/",
		WtDir:     wtDir,
	})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if res.OldBranch != branchAuth {
		t.Errorf("OldBranch = %q, want %q", res.OldBranch, branchAuth)
	}
	if res.NewBranch != "bugfix/auth" {
		t.Errorf("NewBranch = %q, want %q", res.NewBranch, "bugfix/auth")
	}
}

func TestRenameWorktreeTaskAndType(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	// feature/auth → bugfix/login (different task and prefix)
	wt := resolver.WorktreeInfo{Branch: branchAuth, Path: "/wt/feature-auth"}
	res, err := RenameWorktree(context.Background(), r, RenameParams{
		WT:        wt,
		NewTask:   "login",
		NewPrefix: "bugfix/",
		WtDir:     wtDir,
	})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if res.NewBranch != "bugfix/login" {
		t.Errorf("NewBranch = %q, want %q", res.NewBranch, "bugfix/login")
	}
}

type pushRenameMockOpts struct {
	remoteExists bool
	hasUpstream  bool
	// upstreamRemote defaults to git.DefaultRemote when empty.
	upstreamRemote string
	pushErr        error
	deleteErr      error
}

// upstreamCheckDir/pushDir let tests assert the pre-move vs. post-move ordering.
type pushRenameCalls struct {
	pushArgs         []string
	deleteArgs       []string
	upstreamCheckDir string
	pushDir          string
}

func buildPushRenameRunner(opts pushRenameMockOpts, calls *pushRenameCalls) *mockRunner {
	return &mockRunner{
		run:      pushRenameRunFn(opts, calls),
		runInDir: pushRenameRunInDirFn(opts, calls),
	}
}

func pushRenameRunFn(opts pushRenameMockOpts, calls *pushRenameCalls) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == cmdRevParse {
			return "", errGitFailed // BranchExists returns false
		}
		if len(args) >= 2 && args[0] == gitCmdRemote && args[1] == gitSubcmdGetURL {
			if opts.remoteExists {
				return remoteURLStub, nil
			}
			return "", errNoSuchRemote
		}
		if len(args) >= 1 && args[0] == gitCmdPush {
			calls.deleteArgs = args
			return "", opts.deleteErr
		}
		return "", nil
	}
}

func pushRenameRunInDirFn(opts pushRenameMockOpts, calls *pushRenameCalls) func(dir string, args ...string) (string, error) {
	return func(dir string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == cmdRevParse {
			calls.upstreamCheckDir = dir
			if opts.hasUpstream {
				remote := opts.upstreamRemote
				if remote == "" {
					remote = git.DefaultRemote
				}
				return remote + "/" + branchFeature, nil
			}
			return "", errGitFailed
		}
		if len(args) >= 1 && args[0] == gitCmdPush {
			calls.pushArgs = args
			calls.pushDir = dir
			return "", opts.pushErr
		}
		return "", nil
	}
}

func TestRenameWorktreePushSuccess(t *testing.T) {
	calls := &pushRenameCalls{}
	r := buildPushRenameRunner(pushRenameMockOpts{remoteExists: true, hasUpstream: true}, calls)

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir, Push: true})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if calls.pushArgs == nil {
		t.Fatal("expected push -u to run")
	}
	if len(calls.pushArgs) < 4 || calls.pushArgs[1] != "-u" || calls.pushArgs[3] != branchAuth {
		t.Errorf("push args = %v, want [push -u origin %s]", calls.pushArgs, branchAuth)
	}
	if calls.deleteArgs == nil {
		t.Fatal("expected push --delete to run")
	}
	if len(calls.deleteArgs) < 4 || calls.deleteArgs[2] != "--delete" || calls.deleteArgs[3] != branchFeature {
		t.Errorf("delete args = %v, want [push origin --delete %s]", calls.deleteArgs, branchFeature)
	}
	if !res.Published {
		t.Error("expected Published = true")
	}
	if !res.RemoteDeleted {
		t.Error("expected RemoteDeleted = true")
	}
	if calls.upstreamCheckDir != pathWtFeatureLogin {
		t.Errorf("upstream check ran in %q, want the pre-move path %q", calls.upstreamCheckDir, pathWtFeatureLogin)
	}
	if calls.pushDir != res.NewPath {
		t.Errorf("push -u ran in %q, want the post-move path %q", calls.pushDir, res.NewPath)
	}
}

func TestRenameWorktreePushNoUpstream(t *testing.T) {
	calls := &pushRenameCalls{}
	r := buildPushRenameRunner(pushRenameMockOpts{remoteExists: true, hasUpstream: false}, calls)

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir, Push: true})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if calls.pushArgs == nil {
		t.Error("expected push -u to run")
	}
	if calls.deleteArgs != nil {
		t.Error("expected delete to never be invoked when there was no upstream")
	}
	if !res.RemoteSkipped {
		t.Error("expected RemoteSkipped = true")
	}
	if res.RemoteDeleted {
		t.Error("expected RemoteDeleted = false")
	}
	if !res.Published {
		t.Error("expected Published = true")
	}
}

func TestRenameWorktreePushUpstreamOnOtherRemote(t *testing.T) {
	calls := &pushRenameCalls{}
	opts := pushRenameMockOpts{
		remoteExists:   true,
		hasUpstream:    true,
		upstreamRemote: "upstream",
	}
	r := buildPushRenameRunner(opts, calls)

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir, Push: true})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if calls.pushArgs == nil {
		t.Error("expected push -u to run")
	}
	if calls.deleteArgs != nil {
		t.Error("expected delete to never be invoked when the upstream is on a different remote")
	}
	if !res.RemoteSkipped {
		t.Error("expected RemoteSkipped = true when the upstream isn't on git.DefaultRemote")
	}
	if res.RemoteDeleted {
		t.Error("expected RemoteDeleted = false")
	}
	if !res.Published {
		t.Error("expected Published = true")
	}
}

func TestRenameWorktreePushOldRemoteAlreadyGone(t *testing.T) {
	calls := &pushRenameCalls{}
	opts := pushRenameMockOpts{
		remoteExists: true,
		hasUpstream:  true,
		deleteErr:    errors.New("error: remote ref does not exist"),
	}
	r := buildPushRenameRunner(opts, calls)

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir, Push: true})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if !res.RemoteDeleted {
		t.Error("expected RemoteDeleted = true for an already-gone remote branch (idempotent delete)")
	}
	if res.RemoteError != nil {
		t.Errorf("expected nil RemoteError, got %v", res.RemoteError)
	}
}

func TestRenameWorktreePushPublishFails(t *testing.T) {
	calls := &pushRenameCalls{}
	opts := pushRenameMockOpts{
		remoteExists: true,
		hasUpstream:  true,
		pushErr:      errors.New("connection refused"),
	}
	r := buildPushRenameRunner(opts, calls)

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir, Push: true})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if res.PublishError == nil {
		t.Error("expected PublishError to be set")
	}
	if res.Published {
		t.Error("expected Published = false")
	}
	if calls.deleteArgs != nil {
		t.Error("expected delete to never be invoked after a failed publish (old remote branch preserved)")
	}
}

func TestRenameWorktreePushNoOriginRemote(t *testing.T) {
	calls := &pushRenameCalls{}
	r := buildPushRenameRunner(pushRenameMockOpts{remoteExists: false, hasUpstream: true}, calls)

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir, Push: true})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if !res.NoOriginRemote {
		t.Error("expected NoOriginRemote = true")
	}
	if calls.pushArgs != nil {
		t.Error("expected no push -u call when there is no origin remote")
	}
	if calls.deleteArgs != nil {
		t.Error("expected no delete call when there is no origin remote")
	}
}

func TestRenameWorktreeNoPushMakesNoRemoteCalls(t *testing.T) {
	runInDirCalled := false
	calls := &pushRenameCalls{}
	inner := buildPushRenameRunner(pushRenameMockOpts{remoteExists: true, hasUpstream: true}, calls)
	r := &mockRunner{
		run: inner.run,
		runInDir: func(dir string, args ...string) (string, error) {
			runInDirCalled = true
			return inner.runInDir(dir, args...)
		},
	}

	wt := resolver.WorktreeInfo{Branch: branchFeature, Path: pathWtFeatureLogin}
	res, err := RenameWorktree(context.Background(), r, RenameParams{WT: wt, NewTask: "auth", WtDir: wtDir})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if calls.pushArgs != nil || calls.deleteArgs != nil {
		t.Error("expected zero remote/push commands when Push=false")
	}
	if runInDirCalled {
		t.Error("expected zero RunInDir calls when Push=false (HasUpstream/PushSetUpstream must be skipped)")
	}
	if res.Published || res.RemoteDeleted || res.RemoteSkipped || res.NoOriginRemote {
		t.Error("expected all push-related result fields to remain zero-value when Push=false")
	}
}

func TestRenameWorktreeMonorepoTypeOnly(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	// Monorepo: auth-api/feature/auth → auth-api/bugfix/auth (service preserved)
	wt := resolver.WorktreeInfo{Branch: "auth-api/feature/auth", Path: "/wt/auth-api-feature-auth"}
	res, err := RenameWorktree(context.Background(), r, RenameParams{
		WT:        wt,
		NewTask:   "auth",
		NewPrefix: "bugfix/",
		WtDir:     wtDir,
	})
	if err != nil {
		t.Fatalf("RenameWorktree: %v", err)
	}
	if res.NewBranch != "auth-api/bugfix/auth" {
		t.Errorf("NewBranch = %q, want %q", res.NewBranch, "auth-api/bugfix/auth")
	}
}
