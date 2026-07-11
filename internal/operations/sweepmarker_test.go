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
// pointer file resolves to a real admin dir, mirroring `git worktree add`.
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
	if len(m.AdminDirs) != 1 || m.AdminDirs[0].Path != adminDir {
		t.Errorf("AdminDirs = %+v, want [{Path: %s}]", m.AdminDirs, adminDir)
	}
}

func TestWriteSweepManifestSkipsUnreadableGit(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	// A worktree candidate with no .git pointer file at all, e.g. a
	// prunable worktree whose .git has already been removed.
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

// writeManifestFixture plants a manifest for an arbitrary PID, capturing
// each admin dir's real inode to match production behavior.
func writeManifestFixture(t *testing.T, commonDir string, pid int, adminDirs []string) {
	t.Helper()
	writeManifestFixtureWithStart(t, commonDir, pid, adminDirs, time.Now().Add(time.Second).UnixNano())
}

func writeManifestFixtureWithStart(t *testing.T, commonDir string, pid int, adminDirs []string, startUnixNano int64) {
	t.Helper()
	records := make([]adminDirRecord, len(adminDirs))
	for i, dir := range adminDirs {
		ino, hasIno := dirIno(dir)
		records[i] = adminDirRecord{Path: dir, Ino: ino, HasIno: hasIno}
	}
	writeManifestFixtureWithRecords(t, commonDir, pid, records, startUnixNano)
}

// writeManifestFixtureWithStaleIno plants a manifest with a deliberately
// wrong inode for adminDir, simulating a since-reused worktree basename.
func writeManifestFixtureWithStaleIno(t *testing.T, commonDir string, pid int, adminDir string) {
	t.Helper()
	records := []adminDirRecord{{Path: adminDir, Ino: 999999999, HasIno: true}}
	writeManifestFixtureWithRecords(t, commonDir, pid, records, time.Now().Add(time.Second).UnixNano())
}

func writeManifestFixtureWithRecords(t *testing.T, commonDir string, pid int, records []adminDirRecord, startUnixNano int64) {
	t.Helper()
	sweepsDir := filepath.Join(commonDir, sweepManifestDir)
	if err := os.MkdirAll(sweepsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	m := sweepManifest{PID: pid, StartUnixNano: startUnixNano, AdminDirs: records}
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
// PID-reuse case: an "alive" manifest older than the ceiling must be dropped.
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

// TestReapConfidentLocksSkipsAdminDirWithChangedIdentity guards against
// worktree-basename reuse: a manifest's recorded inode no longer matching wins.
func TestReapConfidentLocksSkipsAdminDirWithChangedIdentity(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")

	writeManifestFixtureWithStaleIno(t, commonDir, testutil.DeadPID(t), adminDir)

	lockPath := writeIndexLock(t, adminDir, MinLockAge+time.Second)

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 0 {
		t.Errorf("removals = %+v, want none (admin dir's inode no longer matches the manifest)", removals)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("expected lock to remain; admin dir identity changed since the manifest was written")
	}
}

// TestReapConfidentLocksReapsDespiteInternalMtimeBump is a regression test:
// a lock created inside adminDir bumps its mtime but must not read as recreated.
func TestReapConfidentLocksReapsDespiteInternalMtimeBump(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	_, adminDir := setupWorktree(t, root, commonDir, "wt-a")

	writeManifestFixture(t, commonDir, testutil.DeadPID(t), []string{adminDir})

	// Written after the manifest, exactly like a real sweep's git
	// subprocess creating index.lock inside its own admin dir.
	lockPath := writeIndexLock(t, adminDir, MinLockAge+time.Second)

	removals := ReapConfidentLocks(commonDir)
	if len(removals) != 1 || removals[0].Path != lockPath {
		t.Fatalf("removals = %+v, want exactly [%s] (same dir, only its mtime changed)", removals, lockPath)
	}
}

func TestClassifySweepManifestsGlobError(t *testing.T) {
	commonDir := filepath.Join(t.TempDir(), "unmatched[bracket")
	deadDirs, aliveDirs, deadManifests := classifySweepManifests(commonDir)
	if deadDirs != nil || aliveDirs != nil || deadManifests != nil {
		t.Errorf("classifySweepManifests = (%v, %v, %v), want all nil on a Glob error", deadDirs, aliveDirs, deadManifests)
	}
}

// TestReapConfidentLocksHandlesMissingAdminDir covers a dead-owner manifest
// naming an admin dir that no longer exists — must not crash.
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

// TestReadSweepManifestReadError covers a read error distinct from the
// JSON-parse error already covered by TestReapConfidentLocksIgnoresTornManifest.
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
