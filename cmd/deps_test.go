package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
)

func TestDepsStatusSuccess(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create lockfiles in both dirs so module detection works
	_ = os.WriteFile(filepath.Join(repoDir, deps.LockfilePnpm), []byte("lock-main"), 0644)
	_ = os.WriteFile(filepath.Join(worktreeDir, deps.LockfilePnpm), []byte("lock-feature"), 0644)

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + worktreeDir,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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

	autoDetect := true
	cfg := &config.Config{
		DefaultSource: branchMain,
		WorktreeDir:   "worktrees",
		Deps:          &config.DepsConfig{AutoDetect: &autoDetect},
	}

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := depsStatusCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("depsStatusCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "main") {
		t.Errorf("output missing 'main' branch, got %q", out)
	}
	if !strings.Contains(out, "feature/login") {
		t.Errorf("output missing 'feature/login' branch, got %q", out)
	}
	if !strings.Contains(out, deps.DirNodeModules) {
		t.Errorf("output missing module dir %q, got %q", deps.DirNodeModules, out)
	}
}

func TestDepsStatusNoModules(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// No lockfiles created — no modules should be detected

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + worktreeDir,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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

	autoDetect := true
	cfg := &config.Config{
		DefaultSource: branchMain,
		WorktreeDir:   "worktrees",
		Deps:          &config.DepsConfig{AutoDetect: &autoDetect},
	}

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := depsStatusCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("depsStatusCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "(no modules detected)") {
		t.Errorf("output missing '(no modules detected)', got %q", out)
	}
}

func TestDepsInstallSuccess(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := filepath.Join(repoDir, "worktrees", "feature-login")
	sourceDir := repoDir

	_ = os.MkdirAll(worktreeDir, 0755)

	// Create lockfile in both source and target with same content so clone matches
	_ = os.WriteFile(filepath.Join(sourceDir, deps.LockfilePnpm), []byte("lockfile-content"), 0644)
	_ = os.WriteFile(filepath.Join(worktreeDir, deps.LockfilePnpm), []byte("lockfile-content"), 0644)

	// Create node_modules in source so it can be cloned
	_ = os.MkdirAll(filepath.Join(sourceDir, deps.DirNodeModules), 0755)
	_ = os.WriteFile(filepath.Join(sourceDir, deps.DirNodeModules, "package.json"), []byte("{}"), 0644)

	worktreeOut := strings.Join([]string{
		wtPrefix + sourceDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + worktreeDir,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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

	autoDetect := true
	cfg := &config.Config{
		DefaultSource: branchMain,
		WorktreeDir:   "worktrees",
		Deps:          &config.DepsConfig{AutoDetect: &autoDetect},
	}

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := depsInstallCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("depsInstallCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Dependencies for") {
		t.Errorf("output missing 'Dependencies for', got %q", out)
	}
}

func TestDepsInstallNotFound(t *testing.T) {
	repoDir := t.TempDir()

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
	}, "\n")

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

	autoDetect := true
	cfg := &config.Config{
		DefaultSource: branchMain,
		WorktreeDir:   "worktrees",
		Deps:          &config.DepsConfig{AutoDetect: &autoDetect},
	}

	cmd, _ := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := depsInstallCmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing worktree")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("error = %q, want 'worktree not found'", err.Error())
	}
}

func TestDepsInstallNoModules(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := filepath.Join(repoDir, "worktrees", "feature-login")
	_ = os.MkdirAll(worktreeDir, 0755)

	// No lockfiles — no modules to detect

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + worktreeDir,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

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

	autoDetect := true
	cfg := &config.Config{
		DefaultSource: branchMain,
		WorktreeDir:   "worktrees",
		Deps:          &config.DepsConfig{AutoDetect: &autoDetect},
	}

	cmd, buf := newTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := depsInstallCmd.RunE(cmd, []string{"login"})
	if err != nil {
		t.Fatalf("depsInstallCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No modules detected") {
		t.Errorf("output missing 'No modules detected', got %q", out)
	}
}
