package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
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

// TestClassifyInterruptedWorktree covers classifyInterruptedWorktree's
// decision table directly: which worktrees short-circuit before a status
// call, and how the porcelain classification result is turned into a hit.
func TestClassifyInterruptedWorktree(t *testing.T) {
	cases := []struct {
		name             string
		bare             bool
		prunable         bool
		mainWorktreeGit  bool
		writeIndexLock   bool
		statusOutput     string
		statusErr        error
		wantOK           bool
		wantDeletedCount int
	}{
		{
			name:             "all unstaged deletions is interrupted",
			statusOutput:     " D a.txt\n D b.txt\n",
			wantOK:           true,
			wantDeletedCount: 2,
		},
		{
			name:           "index.lock present defers to the lock-scan flow",
			writeIndexLock: true,
			statusOutput:   " D a.txt\n",
			wantOK:         false,
		},
		{
			name:         "a modified file mixed in is not interrupted",
			statusOutput: " M a.txt\n",
			wantOK:       false,
		},
		{
			name:   "bare worktree is skipped before status is checked",
			bare:   true,
			wantOK: false,
		},
		{
			name:     "prunable worktree is skipped before status is checked",
			prunable: true,
			wantOK:   false,
		},
		{
			name:            "main worktree's directory .git excludes it",
			mainWorktreeGit: true,
			wantOK:          false,
		},
		{
			name:      "a per-worktree status error is swallowed",
			statusErr: errGitFailed,
			wantOK:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			commonDir := filepath.Join(root, "common")

			var wt string
			switch {
			case tc.bare, tc.prunable:
				wt = filepath.Join(root, "wt-skip")
			case tc.mainWorktreeGit:
				wt = root
				if err := os.MkdirAll(filepath.Join(wt, ".git"), 0o755); err != nil {
					t.Fatalf("MkdirAll .git dir: %v", err)
				}
			default:
				var adminDir string
				wt, adminDir = setupWorktree(t, root, commonDir, "wt-a")
				if tc.writeIndexLock {
					writeIndexLock(t, adminDir, 0)
				}
			}

			entry := git.WorktreeEntry{Path: wt, Branch: branchFeature, Bare: tc.bare, Prunable: tc.prunable}
			r := &mockRunner{
				runInDir: func(_ string, _ ...string) (string, error) {
					return tc.statusOutput, tc.statusErr
				},
			}

			got, ok := classifyInterruptedWorktree(context.Background(), r, entry)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (result: %+v)", ok, tc.wantOK, got)
			}
			if !ok {
				return
			}
			if got.DeletedCount != tc.wantDeletedCount {
				t.Errorf("DeletedCount = %d, want %d", got.DeletedCount, tc.wantDeletedCount)
			}
			if got.Path != wt || got.Branch != branchFeature {
				t.Errorf("got Path/Branch = %s/%s, want %s/%s", got.Path, got.Branch, wt, branchFeature)
			}
		})
	}
}

// TestScanInterruptedWorktreesReturnsInterruptedEntries covers the full
// ListWorktrees → classify plumbing, not just the per-entry decision table.
func TestScanInterruptedWorktreesReturnsInterruptedEntries(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	wt, _ := setupWorktree(t, root, commonDir, "wt-a")

	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return porcelainEntries(struct{ path, branch string }{wt, branchFeature}), nil
		},
		runInDir: func(dir string, _ ...string) (string, error) {
			if dir != wt {
				t.Fatalf("RunInDir dir = %s, want %s", dir, wt)
			}
			return " D a.txt\n D b.txt\n", nil
		},
	}

	got, err := ScanInterruptedWorktrees(context.Background(), r)
	if err != nil {
		t.Fatalf("ScanInterruptedWorktrees: %v", err)
	}
	want := []InterruptedWorktree{{Path: wt, Branch: branchFeature, DeletedCount: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestScanInterruptedWorktreesListWorktreesError confirms a ListWorktrees
// failure is the one error that propagates as a hard error from the scan.
func TestScanInterruptedWorktreesListWorktreesError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errGitFailed
		},
		runInDir: noopRunInDir,
	}

	_, err := ScanInterruptedWorktrees(context.Background(), r)
	if !errors.Is(err, errGitFailed) {
		t.Fatalf("ScanInterruptedWorktrees error = %v, want %v", err, errGitFailed)
	}
}
