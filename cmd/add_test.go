package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	prCmdAuth = "auth"
	prCmdPR   = "pr"

	sameRepoPRJSON = `{
  "number": 42,
  "title": "Fix login redirect",
  "headRefName": "fix-login-redirect",
  "headRepository": {"name": "rimba"},
  "headRepositoryOwner": {"login": "lugassawan"},
  "isCrossRepository": false
}`
)

// makeWorktreeGitRunner builds a mockRunner that simulates a successful worktree creation.
// Handles: git common-dir, show-toplevel, fetch, rev-parse (branch absent), worktree add.
func makeWorktreeGitRunner(repoDir string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			switch {
			case len(args) >= 2 && args[1] == cmdGitCommonDir:
				return filepath.Join(repoDir, ".git"), nil
			case len(args) >= 2 && args[1] == cmdShowToplevel:
				return repoDir, nil
			case len(args) >= 1 && args[0] == cmdFetch:
				return "", nil
			case len(args) >= 1 && args[0] == cmdRevParse:
				return "", errGitFailed
			case len(args) >= 1 && args[0] == cmdWorktreeTest:
				if len(args) >= 2 && args[1] == gitSubcmdWorktreeAdd {
					_ = os.MkdirAll(args[4], 0o755)
				}
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
}

// makeOKGhRunner builds a mockGhRunner that returns a successful auth and the given PR JSON.
func makeOKGhRunner(prJSON string) *mockGhRunner {
	return &mockGhRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == prCmdAuth {
				return []byte("Logged in"), nil
			}
			return []byte(prJSON), nil
		},
	}
}

func TestAddSuccess(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

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
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"my-task"})
	if err != nil {
		t.Fatalf("addCmd.RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Created worktree") {
		t.Errorf("output = %q, want 'Created worktree'", out)
	}
	if !strings.Contains(out, "my-task") {
		t.Errorf("output = %q, want 'my-task'", out)
	}
}

func TestAddBranchAlreadyExists(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
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
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"existing"})
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists'", err.Error())
	}
}

func TestAddWithSource(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSource, "develop")
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"from-develop"})
	if err != nil {
		t.Fatalf("addCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "Created worktree") {
		t.Errorf("output = %q, want 'Created worktree'", buf.String())
	}
}

func TestAddPRCmd(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, ".worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{
		WorktreeDir: ".worktrees",
		Deps:        &config.DepsConfig{Modules: []config.ModuleConfig{{Dir: "frontend", Install: "npm install"}}},
	}

	fakeGhDir := t.TempDir()
	fakeGh := filepath.Join(fakeGhDir, "gh")
	_ = os.WriteFile(fakeGh, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeGhDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	restoreGH := overrideGHRunner(makeOKGhRunner(sameRepoPRJSON))
	defer restoreGH()
	restoreRunner := overrideNewRunner(makeWorktreeGitRunner(repoDir))
	defer restoreRunner()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.Flags().String(flagTask, "", "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"pr:42"})
	if err != nil {
		t.Fatalf("addCmd.RunE pr:42: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "PR #42") {
		t.Errorf("output = %q, want 'PR #42'", out)
	}
}

func TestAddPRCmdError(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{WorktreeDir: ".worktrees"}

	fakeGhDir := t.TempDir()
	fakeGh := filepath.Join(fakeGhDir, "gh")
	_ = os.WriteFile(fakeGh, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeGhDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ghR := &mockGhRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return nil, errors.New("gh: not authenticated")
		},
	}
	restoreGH := overrideGHRunner(ghR)
	defer restoreGH()

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restoreRunner := overrideNewRunner(r)
	defer restoreRunner()

	cmd, _ := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.Flags().String(flagTask, "", "")
	addPrefixFlags(cmd)
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"pr:42"})
	if err == nil {
		t.Fatal("expected error from PR add with auth failure")
	}
}

func TestAddWithFeaturePrefix(t *testing.T) {
	repoDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoDir, ".worktrees"), 0755)
	cfg := &config.Config{WorktreeDir: ".worktrees"}

	restore := overrideNewRunner(makeWorktreeGitRunner(repoDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"feature/my-task"})
	if err != nil {
		t.Fatalf("addCmd.RunE feature/my-task: %v", err)
	}
	if !strings.Contains(buf.String(), "my-task") {
		t.Errorf("output = %q, want 'my-task'", buf.String())
	}
}

func TestAddWithService(t *testing.T) {
	repoDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(repoDir, "api"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, ".worktrees"), 0755)
	cfg := &config.Config{WorktreeDir: ".worktrees"}

	restore := overrideNewRunner(makeWorktreeGitRunner(repoDir))
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := addCmd.RunE(cmd, []string{"api/my-task"})
	if err != nil {
		t.Fatalf("addCmd.RunE api/my-task: %v", err)
	}
	if !strings.Contains(buf.String(), "service: api") {
		t.Errorf("output = %q, want 'service: api'", buf.String())
	}
}
