package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestDoctorAutoReapsConfidentDeadOwnerSweepLock guards #383: a stray
// index.lock left by a sweep whose whole rimba process was killed (not just
// its git subprocess, #380's scenario) is recovered automatically — no
// --fix needed — because the sweep manifest proves the owning PID is dead.
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

// TestDoctorSkipsConfidentAliveOwnerSweepLock is the counterpart: when the
// sweep manifest's owner PID is still alive, the lock is left completely
// untouched — not even under `doctor --fix --force` — because it may still
// belong to an in-flight sweep.
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

// TestCleanMergedAutoReapsConfidentDeadOwnerSweepLock guards #383's primary
// reported scenario directly: a `rimba clean --merged` sweep whose whole
// process was killed (not just its git subprocess) leaves stale
// index.lock files that the next `clean --merged` run recovers on its own,
// with no `doctor --fix` needed.
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

// plantSweepManifest writes a sweep manifest at
// <repo>/.git/rimba/sweeps/sweep-<pid>.json naming adminDir as pid's sole
// candidate, mirroring what writeSweepManifest produces in production.
// adminDir is resolved to its canonical form first: git itself canonicalizes
// symlinked paths (e.g. macOS's /var -> /private/var) when it reports
// --git-common-dir, so the manifest must record the same canonical string
// ReapConfidentLocks will compare against, not the raw t.TempDir() path.
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

	// StartUnixNano must be at or after the admin dir's own mtime (already
	// created by rimba add) and within aliveMarkerCeiling, or
	// classifySweepManifests' recreation/ceiling guards will reject it.
	startUnixNano := time.Now().Add(time.Second).UnixNano()
	body := fmt.Sprintf(`{"pid":%d,"start_unix_nano":%d,"admin_dirs":[%q]}`, pid, startUnixNano, realAdminDir)
	path := filepath.Join(sweepsDir, fmt.Sprintf("sweep-%d.json", pid))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write sweep manifest: %v", err)
	}
	return path
}
