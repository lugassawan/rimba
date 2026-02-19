package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

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
	cmd.Flags().BoolP("force", "f", false, "")
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
	cmd.Flags().BoolP("force", "f", false, "")
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
	cmd.Flags().BoolP("force", "f", false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := renameCmd.RunE(cmd, []string{"nonexistent", "new-name"})
	if err == nil {
		t.Fatal("expected error for missing worktree")
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
	cmd.Flags().BoolP("force", "f", false, "")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := renameCmd.RunE(cmd, []string{"login", "auth"})
	if err == nil {
		t.Fatal("expected error from move failure")
	}
}
