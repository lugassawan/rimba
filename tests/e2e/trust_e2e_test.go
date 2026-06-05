package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/lugassawan/rimba/testutil"
)

// writeTeamConfig writes settings.toml to .rimba/ for the given repo.
func writeTeamConfig(t *testing.T, repo string, cfg *config.Config) {
	t.Helper()
	p := filepath.Join(repo, ".rimba", config.TeamFile)
	if err := config.Save(p, cfg); err != nil {
		t.Fatalf("write team config: %v", err)
	}
}

// AC1: fresh repo with post_create — rimba add WITHOUT consent does NOT execute the command.
func TestTrustAC1FreshRepoDoesNotRunWithoutConsent(t *testing.T) {
	repo := setupCleanInitializedRepo(t)

	// Write a post_create hook that would create a marker file.
	markerFile := filepath.Join(repo, "hook-ran")
	cfg := &config.Config{
		WorktreeDir:   "../rimba-trust-wt",
		DefaultSource: "main",
		PostCreate:    []string{"touch " + markerFile},
	}
	writeTeamConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add post_create hook")

	r := rimbaFail(t, repo, "add", "my-task", "--skip-deps")

	// The hook must NOT have run.
	assertFileNotExists(t, markerFile)

	// Error output should mention rimba trust.
	combined := r.Stdout + r.Stderr
	if !strings.Contains(combined, "rimba trust") {
		t.Errorf("error should mention 'rimba trust', got:\nstdout: %s\nstderr: %s", r.Stdout, r.Stderr)
	}
}

// AC3: RIMBA_TRUST_YES=1 auto-approves and the command runs.
func TestTrustAC3EnvYesRunsCommand(t *testing.T) {
	repo := setupCleanInitializedRepo(t)

	markerFile := filepath.Join(repo, "hook-ran")
	cfg := &config.Config{
		WorktreeDir:   "../rimba-trust-wt",
		DefaultSource: "main",
		PostCreate:    []string{"touch " + markerFile},
	}
	writeTeamConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add post_create hook")

	rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "add", "my-task", "--skip-deps")

	assertFileExists(t, markerFile)
}

// AC3: --yes flag auto-approves and the command runs.
func TestTrustAC3FlagYesRunsCommand(t *testing.T) {
	repo := setupCleanInitializedRepo(t)

	markerFile := filepath.Join(repo, "hook-ran")
	cfg := &config.Config{
		WorktreeDir:   "../rimba-trust-wt",
		DefaultSource: "main",
		PostCreate:    []string{"touch " + markerFile},
	}
	writeTeamConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add post_create hook")

	// --yes alone (without RIMBA_TRUST_YES) should be sufficient to auto-approve.
	rimbaSuccess(t, repo, "add", "my-task", "--skip-deps", "--yes")

	assertFileExists(t, markerFile)
}

// AC2: after approval, changing post_create re-arms the gate (denied again).
func TestTrustAC2ChangedCommandReArmsGate(t *testing.T) {
	repo := setupCleanInitializedRepo(t)

	marker1 := filepath.Join(repo, "hook1-ran")
	cfg := &config.Config{
		WorktreeDir:   "../rimba-trust-wt",
		DefaultSource: "main",
		PostCreate:    []string{"touch " + marker1},
	}
	writeTeamConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add post_create hook")

	// Approve and run.
	rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "add", "task-v1", "--skip-deps")
	assertFileExists(t, marker1)

	// Now change the post_create command.
	marker2 := filepath.Join(repo, "hook2-ran")
	cfg.PostCreate = []string{"touch " + marker2}
	writeTeamConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "change post_create hook")

	// Re-arm: add without RIMBA_TRUST_YES should be denied.
	r := rimbaFail(t, repo, "add", "task-v2", "--skip-deps")
	// New hook must NOT have run.
	assertFileNotExists(t, marker2)

	combined := r.Stdout + r.Stderr
	if !strings.Contains(combined, "rimba trust") {
		t.Errorf("re-armed error should mention 'rimba trust', got:\nstdout: %s\nstderr: %s", r.Stdout, r.Stderr)
	}
}

// rimba trust command: approve via rimba trust, then add runs without prompt.
func TestTrustTrustCommandApprovesThenAddRuns(t *testing.T) {
	repo := setupCleanInitializedRepo(t)

	markerFile := filepath.Join(repo, "trust-approved")
	cfg := &config.Config{
		WorktreeDir:   "../rimba-trust-wt",
		DefaultSource: "main",
		PostCreate:    []string{"touch " + markerFile},
	}
	writeTeamConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add post_create hook")

	// Use `rimba trust --yes` to pre-approve.
	rimbaSuccess(t, repo, "trust", "--yes")

	// Now add should run without RIMBA_TRUST_YES.
	rimbaSuccess(t, repo, "add", "approved-task", "--skip-deps")
	assertFileExists(t, markerFile)
}

// rimba trust --show: displays commands and trusted status.
func TestTrustTrustShow(t *testing.T) {
	repo := setupCleanInitializedRepo(t)

	cfg := &config.Config{
		WorktreeDir:   "../rimba-trust-wt",
		DefaultSource: "main",
		PostCreate:    []string{"npm ci", "pnpm build"},
	}
	writeTeamConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add hooks")

	r := rimbaSuccess(t, repo, "trust", "--show")
	assertContains(t, r.Stdout, "npm ci")
	assertContains(t, r.Stdout, "pnpm build")
	assertContains(t, r.Stdout, "sha256:")
	assertContains(t, r.Stdout, "not trusted")
}

// rimba init adds .rimba/*.local.toml glob to .gitignore (covers both
// settings.local.toml and trust.local.toml under a single entry).
func TestTrustInitAddsGitignoreEntry(t *testing.T) {
	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init")

	globEntry := filepath.Join(configDir, localGlob)
	assertGitignoreContains(t, repo, globEntry)
	// Specific trust.local.toml entry must NOT be added (the glob covers it).
	assertGitignoreNotContains(t, repo, filepath.Join(configDir, trust.FileName))
}
