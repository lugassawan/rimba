package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

// TestDoctorReportsAndFixesStaleLock guards #380's operational counterpart:
// a stray index.lock left under <commonDir>/worktrees/<id> (e.g. from a
// killed git process before this fix) is surfaced by `rimba doctor` and
// removed under `--fix --force`.
func TestDoctorReportsAndFixesStaleLock(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskDoctorLock)

	lockPath := plantStaleLock(t, repo)

	r := rimbaSuccess(t, repo, "doctor")
	assertContains(t, r.Stdout, lockPath)

	rimbaSuccess(t, repo, "doctor", "--fix", flagForceE2E)
	assertFileNotExists(t, lockPath)

	r = rimbaSuccess(t, repo, "doctor")
	assertContains(t, r.Stdout, "No stale index.lock files found.")
}

// plantStaleLock creates an index.lock file under the single admin
// subdirectory that `rimba add` created in .git/worktrees, simulating a
// git process that was killed before it could unlink its own lock.
func plantStaleLock(t *testing.T, repo string) string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(repo, ".git", "worktrees"))
	if err != nil {
		t.Fatalf("read .git/worktrees: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 worktree admin dir, got %d: %+v", len(entries), entries)
	}

	lockPath := filepath.Join(repo, ".git", "worktrees", entries[0].Name(), "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatalf("write index.lock: %v", err)
	}

	// Backdate past doctor --fix's minimum-age gate (a fresh lock may still
	// belong to a running git process, so --fix skips it even under --force).
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	return lockPath
}

// TestDoctorAutoReapsConfidentDeadOwnerSweepLock: a stray index.lock left by
// a whole-process-killed sweep is recovered automatically, no --fix needed.
func TestDoctorAutoReapsConfidentDeadOwnerSweepLock(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskDoctorLock)

	lockPath := plantStaleLock(t, repo)
	adminDir := filepath.Dir(lockPath)
	manifestPath := plantSweepManifest(t, repo, testutil.DeadPID(t), adminDir)

	r := rimbaSuccess(t, repo, "doctor")
	assertContains(t, r.Stdout, "Recovered 1 stale index.lock file(s)")
	assertContains(t, r.Stdout, lockPath)
	assertFileNotExists(t, lockPath)
	assertFileNotExists(t, manifestPath)
}

// TestDoctorSkipsConfidentAliveOwnerSweepLock is the counterpart: an alive
// owner PID leaves the lock untouched, even under `doctor --fix --force`.
func TestDoctorSkipsConfidentAliveOwnerSweepLock(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskDoctorLock)

	lockPath := plantStaleLock(t, repo)
	adminDir := filepath.Dir(lockPath)
	plantSweepManifest(t, repo, os.Getpid(), adminDir)

	r := rimbaSuccess(t, repo, "doctor", "--fix", flagForceE2E)
	assertNotContains(t, r.Stdout, "Recovered")
	assertContains(t, r.Stdout, "owned by a sweep that is still running")
	assertFileExists(t, lockPath)
}

// TestCleanMergedAutoReapsConfidentDeadOwnerSweepLock: a `clean --merged`
// sweep killed mid-run leaves a lock the next `clean --merged` recovers.
func TestCleanMergedAutoReapsConfidentDeadOwnerSweepLock(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskDoctorLock)

	lockPath := plantStaleLock(t, repo)
	adminDir := filepath.Dir(lockPath)
	manifestPath := plantSweepManifest(t, repo, testutil.DeadPID(t), adminDir)

	r := rimbaSuccess(t, repo, "clean", flagMergedE2E)
	assertContains(t, r.Stdout, "Recovered 1 stale index.lock file(s)")
	assertFileNotExists(t, lockPath)
	assertFileNotExists(t, manifestPath)
}

// TestDoctorReportsAndFixesInterruptedWorktree guards this branch's core
// scenario (issue #410): a `git worktree remove` killed mid-flight deletes
// every tracked file but never gets to deregister the worktree or touch the
// index, leaving `git status --porcelain` showing 100% unstaged deletions.
// `rimba doctor` must report it with a path + hint, and `--fix --force` must
// finish the removal.
func TestDoctorReportsAndFixesInterruptedWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	wtPath := plantInterruptedWorktree(t, repo, taskDoctorInterrupted)

	r := rimbaSuccess(t, repo, "doctor")
	assertContains(t, r.Stdout, wtPath)
	assertContains(t, r.Stdout, "rimba remove")

	r = rimbaSuccess(t, repo, "doctor", "--fix", flagForceE2E)
	assertContains(t, r.Stdout, "Removed")

	out := testutil.GitCmd(t, repo, "worktree", "list")
	if strings.Contains(out, wtPath) {
		t.Errorf("expected worktree entry for %s to be removed, got: %s", wtPath, out)
	}
}

// plantInterruptedWorktree synthesizes the interrupted-clean signature with
// real git: `rimba add` (skipping deps/hooks so the worktree starts with no
// untracked noise) creates a normal worktree, then its tracked README.md is
// deleted directly from disk — not via `git rm` — the same end state a killed
// `git worktree remove` leaves: the file is gone, the worktree stays
// registered, no index.lock exists, and `git status --porcelain` shows a bare
// unstaged deletion.
func plantInterruptedWorktree(t *testing.T, repo, task string) string {
	t.Helper()

	rimbaSuccess(t, repo, "add", task, flagSkipDepsE2E, flagSkipHooksE2E)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	wtPath := resolver.WorktreePath(wtDir, branch)

	if err := os.Remove(filepath.Join(wtPath, "README.md")); err != nil {
		t.Fatalf("remove tracked file README.md: %v", err)
	}

	return wtPath
}

// plantSweepManifest writes a sweep manifest naming adminDir, resolved to
// its canonical form first since git does the same (e.g. macOS /var -> /private/var).
func plantSweepManifest(t *testing.T, repo string, pid int, adminDir string) string {
	t.Helper()

	realAdminDir, err := filepath.EvalSymlinks(adminDir)
	if err != nil {
		t.Fatalf("EvalSymlinks admin dir: %v", err)
	}

	sweepsDir := filepath.Join(repo, ".git", "rimba", "sweeps")
	if err := os.MkdirAll(sweepsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll sweeps dir: %v", err)
	}

	startUnixNano := time.Now().Add(time.Second).UnixNano()
	body := fmt.Sprintf(`{"pid":%d,"start_unix_nano":%d,"admin_dirs":[{"path":%q}]}`, pid, startUnixNano, realAdminDir)
	path := filepath.Join(sweepsDir, fmt.Sprintf("sweep-%d.json", pid))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write sweep manifest: %v", err)
	}
	return path
}
