package operations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/lugassawan/rimba/testutil"
)

// setupWorktree creates a fake worktree at <root>/wt-<name> whose `.git`
// pointer file resolves to a real admin dir at <commonDir>/worktrees/<name>,
// mirroring what `git worktree add` produces.
func setupWorktree(t *testing.T, root, commonDir, name string) (worktreePath, adminDir string) {
	t.Helper()
	worktreePath = filepath.Join(root, "wt-"+name)
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll worktree: %v", err)
	}
	adminDir = filepath.Join(commonDir, "worktrees", name)
	if err := os.MkdirAll(adminDir, 0o755); err != nil {
		t.Fatalf("MkdirAll admin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: "+adminDir+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .git: %v", err)
	}
	return worktreePath, adminDir
}

func writeIndexLock(t *testing.T, adminDir string, age time.Duration) string {
	t.Helper()
	lockPath := filepath.Join(adminDir, "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile lock: %v", err)
	}
	if age > 0 {
		old := time.Now().Add(-age)
		if err := os.Chtimes(lockPath, old, old); err != nil {
			t.Fatalf("Chtimes: %v", err)
		}
	}
	return lockPath
}

func TestWriteSweepManifestResolvesAdminDirs(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	wt, adminDir := setupWorktree(t, root, commonDir, "wt-a")

	cleanup := writeSweepManifest(commonDir, []string{wt})
	defer cleanup()

	matches, err := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one manifest, got %v (err=%v)", matches, err)
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	var m sweepManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", m.PID, os.Getpid())
	}
	if len(m.AdminDirs) != 1 || m.AdminDirs[0] != adminDir {
		t.Errorf("AdminDirs = %v, want [%s]", m.AdminDirs, adminDir)
	}
}

func TestWriteSweepManifestSkipsUnreadableGit(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	// A worktree candidate with no .git pointer file at all, e.g. a
	// prunable worktree (#374) whose .git has already been removed.
	prunablePath := filepath.Join(root, "wt-gone")
	if err := os.MkdirAll(prunablePath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cleanup := writeSweepManifest(commonDir, []string{prunablePath})
	defer cleanup()

	matches, err := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one manifest even with a skipped candidate, got %v", matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var m sweepManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(m.AdminDirs) != 0 {
		t.Errorf("AdminDirs = %v, want empty (unreadable .git skipped)", m.AdminDirs)
	}
}

func TestWriteSweepManifestCleanupRemovesFile(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	wt, _ := setupWorktree(t, root, commonDir, "wt-a")

	cleanup := writeSweepManifest(commonDir, []string{wt})
	matches, _ := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if len(matches) != 1 {
		t.Fatalf("expected manifest to exist before cleanup, got %v", matches)
	}

	cleanup()

	matches, _ = filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if len(matches) != 0 {
		t.Errorf("expected manifest removed after cleanup, got %v", matches)
	}
}

func TestWriteSweepManifestBestEffortOnUnwritableDir(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	if err := os.MkdirAll(commonDir, 0o555); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(commonDir, 0o755) })

	// Must not panic and must return a safe no-op cleanup.
	cleanup := writeSweepManifest(commonDir, nil)
	cleanup()
}

// TestWriteSweepManifestBestEffortWhenAtomicWriteFails covers the
// atomicWriteFile-failure branch specifically: sweepsDir already exists (so
// MkdirAll is a trivial success) but isn't writable, so os.CreateTemp fails.
func TestWriteSweepManifestBestEffortWhenAtomicWriteFails(t *testing.T) {
	commonDir := t.TempDir()
	sweepsDir := filepath.Join(commonDir, sweepManifestDir)
	if err := os.MkdirAll(sweepsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(sweepsDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(sweepsDir, 0o755) })

	cleanup := writeSweepManifest(commonDir, nil)
	cleanup()

	matches, _ := filepath.Glob(filepath.Join(sweepsDir, "sweep-*.json"))
	if len(matches) != 0 {
		t.Errorf("expected no manifest written, got %v", matches)
	}
}

func TestReapConfidentLocksDeadOwnerReapsInSetAgedLock(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")
	lockPath := writeIndexLock(t, adminDir, MinLockAge+time.Second)

	writeManifestFixture(t, commonDir, testutil.DeadPID(t), []string{adminDir})

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 1 || removals[0].Path != lockPath {
		t.Fatalf("removals = %+v, want exactly [%s]", removals, lockPath)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected lock to be removed")
	}
	assertManifestsGone(t, commonDir)
}

func TestReapConfidentLocksSkipsFreshLockBelowFloor(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")
	lockPath := writeIndexLock(t, adminDir, 0)

	writeManifestFixture(t, commonDir, testutil.DeadPID(t), []string{adminDir})

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 0 {
		t.Errorf("removals = %+v, want none (lock younger than MinLockAge)", removals)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected fresh lock to remain")
	}
}

func TestReapConfidentLocksSkipsAliveOwner(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")
	lockPath := writeIndexLock(t, adminDir, MinLockAge+time.Second)

	writeManifestFixture(t, commonDir, os.Getpid(), []string{adminDir})

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 0 {
		t.Errorf("removals = %+v, want none (owner still alive)", removals)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock to remain when owner is alive")
	}

	matches, _ := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if len(matches) != 1 {
		t.Errorf("expected alive-owner manifest to survive, got %v", matches)
	}
}

func TestReapConfidentLocksIgnoresTornManifest(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")
	lockPath := writeIndexLock(t, adminDir, MinLockAge+time.Second)

	sweepsDir := filepath.Join(commonDir, sweepManifestDir)
	if err := os.MkdirAll(sweepsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sweepsDir, "sweep-999999.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 0 {
		t.Errorf("removals = %+v, want none (torn manifest is fail-soft)", removals)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock to remain; torn manifest grants no confidence")
	}
}

func TestReapConfidentLocksNoManifestsIsNoop(t *testing.T) {
	commonDir := filepath.Join(t.TempDir(), "common")
	if removals := ReapConfidentLocks(commonDir); len(removals) != 0 {
		t.Errorf("removals = %+v, want none", removals)
	}
}

// writeManifestFixture writes a sweep manifest directly, bypassing
// writeSweepManifest, so tests can plant an arbitrary PID (e.g. one
// confirmed dead) without needing a real worktree candidate list.
// StartUnixNano defaults to just after "now" — at or after the admin dir's
// own mtime (the caller already created it via setupWorktree) and within
// aliveMarkerCeiling — since classifySweepManifests' recreation/ceiling
// guards would otherwise reject it. Use writeManifestFixtureWithStart to
// exercise those guards directly.
func writeManifestFixture(t *testing.T, commonDir string, pid int, adminDirs []string) {
	t.Helper()
	writeManifestFixtureWithStart(t, commonDir, pid, adminDirs, time.Now().Add(time.Second).UnixNano())
}

func writeManifestFixtureWithStart(t *testing.T, commonDir string, pid int, adminDirs []string, startUnixNano int64) {
	t.Helper()
	sweepsDir := filepath.Join(commonDir, sweepManifestDir)
	if err := os.MkdirAll(sweepsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	m := sweepManifest{PID: pid, StartUnixNano: startUnixNano, AdminDirs: adminDirs}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := filepath.Join(sweepsDir, "sweep-"+strconv.Itoa(pid)+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func assertManifestsGone(t *testing.T, commonDir string) {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if len(matches) != 0 {
		t.Errorf("expected dead manifests to be cleaned up, got %v", matches)
	}
}

func TestAliveSweepAdminDirsReportsOnlyAliveOwners(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, aliveDir := setupWorktree(t, root, commonDir, "wt-alive")
	_, deadDir := setupWorktree(t, root, commonDir, "wt-dead")

	writeManifestFixture(t, commonDir, os.Getpid(), []string{aliveDir})
	writeManifestFixture(t, commonDir, testutil.DeadPID(t), []string{deadDir})

	alive := AliveSweepAdminDirs(commonDir)
	if !alive[aliveDir] {
		t.Errorf("alive = %v, want %s present", alive, aliveDir)
	}
	if alive[deadDir] {
		t.Errorf("alive = %v, want %s absent (owner is dead)", alive, deadDir)
	}
}

func TestAliveSweepAdminDirsNoManifests(t *testing.T) {
	commonDir := filepath.Join(t.TempDir(), "common")
	if alive := AliveSweepAdminDirs(commonDir); len(alive) != 0 {
		t.Errorf("alive = %v, want empty", alive)
	}
}

// TestAliveSweepAdminDirsIgnoresManifestPastCeiling guards the Windows/
// PID-reuse case: proc.Alive always reports true on Windows, and a PID can
// be reused by an unrelated process, so an "alive" manifest older than
// aliveMarkerCeiling must be dropped rather than permanently vetoing
// doctor --fix for its locks.
func TestAliveSweepAdminDirsIgnoresManifestPastCeiling(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")

	ancientStart := time.Now().Add(-(aliveMarkerCeiling + time.Minute)).UnixNano()
	writeManifestFixtureWithStart(t, commonDir, os.Getpid(), []string{adminDir}, ancientStart)

	alive := AliveSweepAdminDirs(commonDir)
	if alive[adminDir] {
		t.Errorf("alive = %v, want %s absent (manifest past aliveMarkerCeiling)", alive, adminDir)
	}
}

// TestReapConfidentLocksSkipsAdminDirRecreatedSinceManifest guards against
// worktree-basename reuse: if adminDir was recreated (e.g. a new, unrelated
// worktree reusing the same path) after the dead-owner manifest's sweep
// started, that manifest's PID no longer attests to whatever now lives at
// adminDir, so its lock must not be reaped.
func TestReapConfidentLocksSkipsAdminDirRecreatedSinceManifest(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")

	// Manifest claims to predate adminDir's creation — simulating a dead
	// sweep whose manifest still names a path that was later reused by a
	// fresh, unrelated worktree.
	staleStart := time.Now().Add(-time.Hour).UnixNano()
	writeManifestFixtureWithStart(t, commonDir, testutil.DeadPID(t), []string{adminDir}, staleStart)

	lockPath := writeIndexLock(t, adminDir, MinLockAge+time.Second)

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 0 {
		t.Errorf("removals = %+v, want none (admin dir recreated after manifest was written)", removals)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock to remain; admin dir postdates the dead manifest")
	}
}

// TestClassifySweepManifestsGlobError covers the malformed-pattern branch:
// an unmatched '[' in commonDir makes filepath.Glob itself error, and that
// error must fail soft (no panic, no matches) rather than propagate.
func TestClassifySweepManifestsGlobError(t *testing.T) {
	commonDir := filepath.Join(t.TempDir(), "unmatched[bracket")
	deadDirs, aliveDirs, deadManifests := classifySweepManifests(commonDir)
	if deadDirs != nil || aliveDirs != nil || deadManifests != nil {
		t.Errorf("classifySweepManifests = (%v, %v, %v), want all nil on a Glob error", deadDirs, aliveDirs, deadManifests)
	}
}

// TestReapConfidentLocksHandlesMissingAdminDir covers
// adminDirRecreatedSince's os.Stat-error branch: a dead-owner manifest
// naming an admin dir that no longer exists (already removed) must not
// crash and simply finds nothing to reap there.
func TestReapConfidentLocksHandlesMissingAdminDir(t *testing.T) {
	commonDir := t.TempDir()
	missingAdminDir := filepath.Join(commonDir, "worktrees", "wt-gone")
	writeManifestFixture(t, commonDir, testutil.DeadPID(t), []string{missingAdminDir})

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 0 {
		t.Errorf("removals = %+v, want none (admin dir never existed)", removals)
	}
	assertManifestsGone(t, commonDir)
}

// TestReadSweepManifestReadError covers readSweepManifest's os.ReadFile
// error branch (distinct from the JSON-unmarshal-error path already
// covered by TestReapConfidentLocksIgnoresTornManifest): a manifest "file"
// that's actually a directory fails to read, not to parse.
func TestReadSweepManifestReadError(t *testing.T) {
	commonDir := t.TempDir()
	sweepsDir := filepath.Join(commonDir, sweepManifestDir)
	manifestPath := filepath.Join(sweepsDir, "sweep-999999.json")
	if err := os.MkdirAll(manifestPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	manifest, ok := readSweepManifest(manifestPath)
	if ok {
		t.Errorf("readSweepManifest = (%+v, true), want ok=false for an unreadable path", manifest)
	}
}

// TestReadWorktreeAdminDirMalformedGitFile covers the CutPrefix-not-found
// branch: a .git file whose content doesn't start with "gitdir:" (corrupt
// or unexpected format) is treated the same as unreadable — ok=false.
func TestReadWorktreeAdminDirMalformedGitFile(t *testing.T) {
	worktreePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("not a gitdir pointer\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	dir, ok := readWorktreeAdminDir(worktreePath)
	if ok {
		t.Errorf("readWorktreeAdminDir = (%q, true), want ok=false for a malformed .git file", dir)
	}
}
