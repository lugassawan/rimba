package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

// TestInitFreshGitignoreIncludesMetrics asserts a non-personal fresh init
// adds the metrics.jsonl entry to .gitignore alongside the local-config glob.
func TestInitFreshGitignoreIncludesMetrics(t *testing.T) {
	repoDir := t.TempDir()

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	metricsEntry := config.DirName + "/" + metricsFileName
	if !strings.Contains(string(data), metricsEntry) {
		t.Errorf(".gitignore should contain %q, got:\n%s", metricsEntry, data)
	}
}

// TestInitFreshMetricsGitignoreWriteError covers the error path of the new
// metrics.jsonl gitignore call in runInitFresh: the glob entry is already
// present (so ensureLocalIgnore's write is a no-op), but .gitignore is
// read-only so appending the metrics entry fails — and that failure must
// fail rimba init, mirroring the existing gitignore call's failure policy.
func TestInitFreshMetricsGitignoreWriteError(t *testing.T) {
	repoDir := t.TempDir()

	// Remove the worktree dir created outside repoDir (../repoName-worktrees).
	repoName := filepath.Base(repoDir)
	wtDir := filepath.Join(repoDir, "..", repoName+"-worktrees")
	t.Cleanup(func() { os.RemoveAll(wtDir) })

	// Pre-seed .gitignore with the glob entry already present (so the first
	// write is a no-op) and make it read-only.
	globEntry := config.DirName + "/" + config.LocalGlob
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(globEntry+"\n"), 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitignorePath, 0644) })

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when .gitignore is read-only for the metrics entry, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update .gitignore") {
		t.Errorf("error = %q, want 'failed to update .gitignore'", err.Error())
	}
}

// TestInitPersonalGitignoreExcludesMetrics asserts --personal init does NOT
// add a redundant metrics.jsonl entry — the whole .rimba/ dir is already
// ignored.
func TestInitPersonalGitignoreExcludesMetrics(t *testing.T) {
	repoDir := t.TempDir()

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagPersonal, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	metricsEntry := config.DirName + "/" + metricsFileName
	if strings.Contains(string(data), metricsEntry) {
		t.Errorf(".gitignore should not contain %q in personal mode, got:\n%s", metricsEntry, data)
	}
}

// TestInitReInitMetricsGitignoreWriteError covers the error path of the new
// metrics.jsonl gitignore call in reconcileExistingIgnore: the glob entry is
// already present (so the first EnsureLocalGlobIgnored call is a no-op that
// needs no write), but .gitignore is read-only so appending the metrics
// entry fails — and that failure must fail rimba init, mirroring the
// existing gitignore call's failure policy.
func TestInitReInitMetricsGitignoreWriteError(t *testing.T) {
	repoDir := t.TempDir()

	rimbaDir := filepath.Join(repoDir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}

	globEntry := config.DirName + "/" + config.LocalGlob
	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(globEntry+"\n"), 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitignorePath, 0644) })

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when .gitignore is read-only during re-init, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update .gitignore") {
		t.Errorf("error = %q, want 'failed to update .gitignore'", err.Error())
	}
}

// TestInitReInitAddsMetricsGitignore covers the re-init-on-existing-.rimba/
// path (reconcileExistingIgnore) — users upgrading rimba on an
// already-initialized non-personal repo get the metrics entry too.
func TestInitReInitAddsMetricsGitignore(t *testing.T) {
	repoDir := t.TempDir()

	rimbaDir := filepath.Join(repoDir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}

	r := repoRootRunner(repoDir, func(_ ...string) (string, error) { return "", nil })
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "already exists") {
		t.Fatalf("expected re-init path, output:\n%s", buf.String())
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	metricsEntry := config.DirName + "/" + metricsFileName
	if !strings.Contains(string(data), metricsEntry) {
		t.Errorf(".gitignore should contain %q after re-init, got:\n%s", metricsEntry, data)
	}
}
