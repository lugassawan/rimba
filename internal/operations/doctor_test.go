package operations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanWorktreeLocksFindsLocks(t *testing.T) {
	commonDir := t.TempDir()
	for _, wt := range []string{"wt-a", "wt-b"} {
		dir := filepath.Join(commonDir, "worktrees", wt)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.lock"), nil, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	locks, err := ScanWorktreeLocks(commonDir)
	if err != nil {
		t.Fatalf("ScanWorktreeLocks: %v", err)
	}
	if len(locks) != 2 {
		t.Fatalf("expected 2 locks, got %d: %+v", len(locks), locks)
	}
	for _, l := range locks {
		if l.ModTime.IsZero() {
			t.Errorf("expected non-zero ModTime for %s", l.Path)
		}
	}
}

func TestScanWorktreeLocksEmpty(t *testing.T) {
	commonDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(commonDir, "worktrees", "wt-a"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	locks, err := ScanWorktreeLocks(commonDir)
	if err != nil {
		t.Fatalf("ScanWorktreeLocks: %v", err)
	}
	if len(locks) != 0 {
		t.Errorf("expected 0 locks, got %d: %+v", len(locks), locks)
	}
}

func TestScanWorktreeLocksBadPattern(t *testing.T) {
	_, err := ScanWorktreeLocks(filepath.Join(t.TempDir(), "unmatched[bracket"))
	if err == nil {
		t.Fatal("expected error for a malformed glob pattern")
	}
	if !strings.Contains(err.Error(), "scan for stale locks") {
		t.Errorf("expected a friendly wrapped error, got: %v", err)
	}
}

func TestScanWorktreeLocksSkipsUnstatableEntries(t *testing.T) {
	commonDir := t.TempDir()
	wtDir := filepath.Join(commonDir, "worktrees", "wt-a")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// A broken symlink matches the glob but fails os.Stat (which follows
	// symlinks), exercising the same race a lock removed between glob and
	// stat would hit.
	brokenLink := filepath.Join(wtDir, "index.lock")
	if err := os.Symlink(filepath.Join(wtDir, "does-not-exist"), brokenLink); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	locks, err := ScanWorktreeLocks(commonDir)
	if err != nil {
		t.Fatalf("ScanWorktreeLocks: %v", err)
	}
	if len(locks) != 0 {
		t.Errorf("expected unstatable entry to be skipped, got %d locks: %+v", len(locks), locks)
	}
}

func TestScanWorktreeLocksMissingDir(t *testing.T) {
	locks, err := ScanWorktreeLocks(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("ScanWorktreeLocks on a missing dir should not error, got: %v", err)
	}
	if len(locks) != 0 {
		t.Errorf("expected 0 locks, got %d", len(locks))
	}
}

func TestRemoveStaleLocksSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.lock")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	removals := RemoveStaleLocks([]LockInfo{{Path: path}})
	if len(removals) != 1 {
		t.Fatalf("expected 1 removal, got %d", len(removals))
	}
	if removals[0].Err != nil {
		t.Errorf("unexpected error: %v", removals[0].Err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed")
	}
}

func TestRemoveStaleLocksUnremovablePath(t *testing.T) {
	removals := RemoveStaleLocks([]LockInfo{{Path: filepath.Join(t.TempDir(), "missing", "index.lock")}})
	if len(removals) != 1 {
		t.Fatalf("expected 1 removal, got %d", len(removals))
	}
	if removals[0].Err == nil {
		t.Error("expected error removing a nonexistent path")
	}
}
