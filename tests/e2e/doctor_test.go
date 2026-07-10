package e2e_test

import (
	"os"
	"path/filepath"
	"testing"
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
	return lockPath
}
