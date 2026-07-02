package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const remoteURLStub = "https://example.com/repo.git"

func TestRenameSuccess(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := renameCmd.RunE(cmd, []string{"login", "auth"})
	if err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "Renamed worktree") {
		t.Errorf("output = %q, want 'Renamed worktree'", buf.String())
	}
}

func TestRenameBranchAlreadyExists(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", nil // BranchExists returns true
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := renameCmd.RunE(cmd, []string{"login", "auth"})
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestRenameWorktreeNotFound(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := renameCmd.RunE(cmd, []string{"nonexistent", "new-name"})
	if err == nil {
		t.Fatal("expected error for missing worktree")
	}
}

func makeRenameWorktreeOut(repoDir string) string {
	return strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")
}

func makeRenameRunner(repoDir, worktreeOut string) *mockRunner {
	return makeRenamePushRunner(repoDir, worktreeOut, renamePushMockOpts{})
}

// renamePushMockOpts configures makeRenamePushRunner's simulated remote/push responses,
// mirroring pushRenameMockOpts in internal/operations/rename_test.go.
type renamePushMockOpts struct {
	remoteExists bool
	hasUpstream  bool
	pushErr      error
	deleteErr    error
}

// makeRenamePushRunner extends makeRenameRunner's worktree-listing/branch-exists behavior
// with the remote/push subcommands exercised by RenameWorktree's --push path: `remote
// get-url origin` (RemoteExists), `push -u origin <new>` in the new worktree dir
// (PushSetUpstream), and `push origin --delete <old>` (DeleteRemoteBranch).
func makeRenamePushRunner(repoDir, worktreeOut string, opts renamePushMockOpts) *mockRunner {
	return &mockRunner{
		run:      renamePushRunFn(repoDir, worktreeOut, opts),
		runInDir: renamePushRunInDirFn(opts),
	}
}

// renamePushRunFn answers the repo-wide git commands RenameWorktree issues via Run:
// worktree listing/lookup, BranchExists, RemoteExists, and DeleteRemoteBranch.
func renamePushRunFn(repoDir, worktreeOut string, opts renamePushMockOpts) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		if len(args) >= 2 && args[1] == cmdGitCommonDir {
			return filepath.Join(repoDir, ".git"), nil
		}
		if len(args) >= 2 && args[1] == cmdShowToplevel {
			return repoDir, nil
		}
		if len(args) >= 2 && args[0] == cmdRemote && args[1] == "get-url" {
			if opts.remoteExists {
				return remoteURLStub, nil
			}
			return "", errGitFailed
		}
		if len(args) >= 1 && args[0] == cmdRevParse {
			return "", errGitFailed // BranchExists returns false
		}
		if len(args) >= 1 && args[0] == gitCmdPush {
			return "", opts.deleteErr // push origin --delete <old>
		}
		return worktreeOut, nil
	}
}

// renamePushRunInDirFn answers the worktree-directory-scoped git commands RenameWorktree
// issues via RunInDir: HasUpstream and PushSetUpstream.
func renamePushRunInDirFn(opts renamePushMockOpts) func(dir string, args ...string) (string, error) {
	return func(_ string, args ...string) (string, error) {
		if len(args) >= 1 && args[0] == cmdRevParse {
			if opts.hasUpstream {
				return git.DefaultRemote + "/" + branchFeature, nil
			}
			return "", errGitFailed
		}
		if len(args) >= 1 && args[0] == gitCmdPush {
			return "", opts.pushErr // push -u origin <new>
		}
		return "", nil
	}
}

func TestRenameRunsHooksWhenConfigured(t *testing.T) {
	t.Setenv("RIMBA_TRUST_YES", "1")
	// WorktreeDir must be relative; cmd layer joins it with repoRoot.
	// new branch = feature/auth → new path = <repoDir>/wt/feature-auth
	repoDir := t.TempDir()
	newWtPath := filepath.Join(repoDir, "wt", "feature-auth")
	_ = os.MkdirAll(newWtPath, 0755)
	marker := filepath.Join(repoDir, "hook-ran")

	cfg := &config.Config{
		WorktreeDir:   "wt",
		DefaultSource: branchMain,
		PostRename:    []string{"touch " + marker},
	}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	restore := overrideNewRunner(makeRenameRunner(repoDir, worktreeOut))
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		t.Error("post-rename hook did not run; expected marker file to be created")
	}
}

func TestRenameSkipHooks(t *testing.T) {
	t.Setenv("RIMBA_TRUST_YES", "1")
	repoDir := t.TempDir()
	hookDir := t.TempDir()
	marker := filepath.Join(hookDir, "hook-ran")
	cfg := &config.Config{
		WorktreeDir:   hookDir,
		DefaultSource: branchMain,
		PostRename:    []string{"touch " + marker},
	}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	restore := overrideNewRunner(makeRenameRunner(repoDir, worktreeOut))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("post-rename hook must not run when --skip-hooks is set")
	}
	if !strings.Contains(buf.String(), "Renamed worktree") {
		t.Errorf("output = %q, want 'Renamed worktree'", buf.String())
	}
}

func TestRenameSkipDeps(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	restore := overrideNewRunner(makeRenameRunner(repoDir, worktreeOut))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "Renamed worktree") {
		t.Errorf("output = %q, want 'Renamed worktree'", buf.String())
	}
}

func TestRenameMoveFails(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	_ = config.Save(filepath.Join(repoDir, config.FileName), cfg)
	_ = os.MkdirAll(filepath.Join(repoDir, "worktrees"), 0755)

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == "move" {
				return "", errGitFailed
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := renameCmd.RunE(cmd, []string{"login", "auth"})
	if err == nil {
		t.Fatal("expected error from move failure")
	}
}

func TestRenameRetypeFlag(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	restore := overrideNewRunner(makeRenameRunner(repoDir, worktreeOut))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	_ = cmd.Flags().Set("bugfix", "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := renameCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "feature/login -> bugfix/login") {
		t.Errorf("output = %q, want 'feature/login -> bugfix/login'", buf.String())
	}
}

// newRenamePushTestCmd builds a test command with the flags --push relies on
// (--force, --skip-deps, --skip-hooks, --push) registered the same way the real
// init() registers them, with --skip-deps set so PostRenameSetup does no real work.
func newRenamePushTestCmd(cfg *config.Config) (*cobra.Command, *bytes.Buffer) {
	cmd, buf := newTestCmd()
	cmd.Flags().BoolP(flagForce, "f", false, "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.Flags().Bool(flagPush, false, "")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagPush, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))
	return cmd, buf
}

func TestRenamePushPublishAndDeleteSuccess(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	opts := renamePushMockOpts{remoteExists: true, hasUpstream: true}
	restore := overrideNewRunner(makeRenamePushRunner(repoDir, worktreeOut, opts))
	defer restore()

	cmd, buf := newRenamePushTestCmd(cfg)

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Published branch: origin/feature/auth") {
		t.Errorf("output = %q, want 'Published branch: origin/feature/auth'", out)
	}
	if !strings.Contains(out, "Deleted remote branch: origin/feature/login") {
		t.Errorf("output = %q, want 'Deleted remote branch: origin/feature/login'", out)
	}
}

func TestRenamePushNoUpstreamSkipsDeleteQuietly(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	opts := renamePushMockOpts{remoteExists: true, hasUpstream: false}
	restore := overrideNewRunner(makeRenamePushRunner(repoDir, worktreeOut, opts))
	defer restore()

	cmd, buf := newRenamePushTestCmd(cfg)

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Published branch: origin/feature/auth") {
		t.Errorf("output = %q, want 'Published branch: origin/feature/auth'", out)
	}
	if strings.Contains(out, "Deleted remote branch") || strings.Contains(out, "Failed to delete") {
		t.Errorf("output = %q, want no delete-related output when there was no upstream to delete", out)
	}
}

func TestRenamePushPublishFailureShowsRecoveryHint(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	opts := renamePushMockOpts{remoteExists: true, hasUpstream: true, pushErr: errors.New("connection refused")}
	restore := overrideNewRunner(makeRenamePushRunner(repoDir, worktreeOut, opts))
	defer restore()

	cmd, buf := newRenamePushTestCmd(cfg)

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Failed to publish branch feature/auth: connection refused") {
		t.Errorf("output = %q, want publish failure message", out)
	}
	if !strings.Contains(out, "To publish: git push -u origin feature/auth") {
		t.Errorf("output = %q, want 'To publish: git push -u origin feature/auth'", out)
	}
	if strings.Contains(out, "Deleted remote branch") {
		t.Errorf("output = %q, want no delete attempted after a failed publish", out)
	}
}

func TestRenamePushDeleteFailureShowsRecoveryHint(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	opts := renamePushMockOpts{remoteExists: true, hasUpstream: true, deleteErr: errors.New("network error")}
	restore := overrideNewRunner(makeRenamePushRunner(repoDir, worktreeOut, opts))
	defer restore()

	cmd, buf := newRenamePushTestCmd(cfg)

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Published branch: origin/feature/auth") {
		t.Errorf("output = %q, want 'Published branch: origin/feature/auth'", out)
	}
	if !strings.Contains(out, "Failed to delete remote branch origin/feature/login") {
		t.Errorf("output = %q, want 'Failed to delete remote branch origin/feature/login'", out)
	}
	if !strings.Contains(out, "network error") {
		t.Errorf("output = %q, want remote error reason 'network error'", out)
	}
	if !strings.Contains(out, "To delete remote: git push origin --delete feature/login") {
		t.Errorf("output = %q, want 'To delete remote: git push origin --delete feature/login'", out)
	}
}

func TestRenamePushNoOriginRemoteSkipsQuietly(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: defaultRelativeWtDir, DefaultSource: branchMain}
	worktreeOut := makeRenameWorktreeOut(repoDir)
	opts := renamePushMockOpts{remoteExists: false}
	restore := overrideNewRunner(makeRenamePushRunner(repoDir, worktreeOut, opts))
	defer restore()

	cmd, buf := newRenamePushTestCmd(cfg)

	if err := renameCmd.RunE(cmd, []string{"login", "auth"}); err != nil {
		t.Fatalf("renameCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No origin remote; skipped publishing.") {
		t.Errorf("output = %q, want 'No origin remote; skipped publishing.'", out)
	}
	if strings.Contains(out, "Published branch") || strings.Contains(out, "Deleted remote branch") {
		t.Errorf("output = %q, want no publish/delete output when there is no origin remote", out)
	}
}
