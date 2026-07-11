package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/proc"
)

// sweepManifestDir is where sweep manifests live, relative to commonDir.
const sweepManifestDir = "rimba/sweeps"

// aliveMarkerCeiling caps how long an "alive" classification is trusted —
// proc.Alive is always true on Windows, so a stale manifest can't permanently block doctor --fix.
const aliveMarkerCeiling = 5 * time.Minute

// sweepManifest records a sweep's PID, start time, and admin dirs — enough
// for a later run to prove (not guess) that its owner is dead.
type sweepManifest struct {
	PID           int              `json:"pid"`
	StartUnixNano int64            `json:"start_unix_nano"`
	AdminDirs     []adminDirRecord `json:"admin_dirs"`
}

// adminDirRecord pairs an admin dir path with its inode at manifest-write
// time, so a later run can tell it apart from a same-path recreated dir.
type adminDirRecord struct {
	Path   string `json:"path"`
	Ino    uint64 `json:"ino"`
	HasIno bool   `json:"has_ino"`
}

// ReapConfidentLocks removes locks whose admin dir is in a manifest with a
// confirmed-dead owner and an age past MinLockAge; everything else stays on doctor --fix.
func ReapConfidentLocks(commonDir string) []LockRemoval {
	deadAdminDirs, _, deadManifests := classifySweepManifests(commonDir)
	defer removeManifests(deadManifests)

	if len(deadAdminDirs) == 0 {
		return nil
	}

	locks, err := ScanWorktreeLocks(commonDir)
	if err != nil {
		return nil
	}

	var confident []LockInfo
	for _, l := range locks {
		if deadAdminDirs[filepath.Dir(l.Path)] && l.Age >= MinLockAge {
			confident = append(confident, l)
		}
	}

	return RemoveStaleLocks(confident)
}

// AliveSweepAdminDirs returns admin dirs claimed by a confirmed-alive owner,
// so doctor skips them (even under --force) instead of the age-based flow.
func AliveSweepAdminDirs(commonDir string) map[string]bool {
	_, aliveAdminDirs, _ := classifySweepManifests(commonDir)
	return aliveAdminDirs
}

// writeSweepManifest records this sweep's PID/start/admin dirs so a later
// reap can attribute an orphaned lock to it; best-effort, never aborts the caller.
func writeSweepManifest(commonDir string, worktreePaths []string) (cleanup func()) {
	noop := func() {}

	adminDirs := make([]adminDirRecord, 0, len(worktreePaths))
	for _, p := range worktreePaths {
		dir, ok := readWorktreeAdminDir(p)
		if !ok {
			continue
		}
		ino, hasIno := dirIno(dir)
		adminDirs = append(adminDirs, adminDirRecord{Path: dir, Ino: ino, HasIno: hasIno})
	}

	sweepsDir := filepath.Join(commonDir, sweepManifestDir)
	if err := os.MkdirAll(sweepsDir, 0750); err != nil {
		return noop
	}

	manifest := sweepManifest{
		PID:           os.Getpid(),
		StartUnixNano: time.Now().UnixNano(),
		AdminDirs:     adminDirs,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return noop
	}

	path := filepath.Join(sweepsDir, fmt.Sprintf("sweep-%d.json", manifest.PID))
	if err := atomicWriteFile(sweepsDir, path, data); err != nil {
		return noop
	}

	return func() { _ = os.Remove(path) }
}

// deferSweepManifest writes a manifest for paths so its cleanup can be
// deferred at the call site; best-effort no-op on a CommonDir failure.
func deferSweepManifest(ctx context.Context, r git.Runner, paths []string) func() {
	commonDir, err := git.CommonDir(ctx, r)
	if err != nil || !filepath.IsAbs(commonDir) {
		return func() {}
	}
	return writeSweepManifest(commonDir, paths)
}

// classifySweepManifests splits manifests' admin dirs into confirmed-dead
// vs confirmed-alive; torn manifests are skipped (fail-soft).
func classifySweepManifests(commonDir string) (deadDirs, aliveDirs map[string]bool, deadManifests []string) {
	matches, err := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if err != nil {
		return nil, nil, nil
	}

	deadDirs = make(map[string]bool)
	aliveDirs = make(map[string]bool)
	for _, path := range matches {
		manifest, ok := readSweepManifest(path)
		if !ok {
			continue
		}
		if classifyManifest(manifest, deadDirs, aliveDirs) {
			deadManifests = append(deadManifests, path)
		}
	}
	return deadDirs, aliveDirs, deadManifests
}

// classifyManifest routes one manifest's admin dirs into deadDirs/aliveDirs
// via proc.Alive plus the ceiling and identity guards, and reports dead.
func classifyManifest(manifest sweepManifest, deadDirs, aliveDirs map[string]bool) (dead bool) {
	if proc.Alive(manifest.PID) {
		if !manifestPastCeiling(manifest) {
			for _, entry := range manifest.AdminDirs {
				aliveDirs[entry.Path] = true
			}
		}
		return false
	}

	for _, entry := range manifest.AdminDirs {
		if !adminDirIdentityChanged(entry) {
			deadDirs[entry.Path] = true
		}
	}
	return true
}

// manifestPastCeiling reports whether manifest was written more than
// aliveMarkerCeiling ago.
func manifestPastCeiling(manifest sweepManifest) bool {
	return time.Since(time.Unix(0, manifest.StartUnixNano)) > aliveMarkerCeiling
}

// adminDirIdentityChanged reports whether entry's dir was removed and a new,
// unrelated one created at the same path (an internal mtime bump doesn't count).
func adminDirIdentityChanged(entry adminDirRecord) bool {
	if !entry.HasIno {
		return false
	}
	ino, ok := dirIno(entry.Path)
	return ok && ino != entry.Ino
}

// readSweepManifest parses the manifest at path, reporting ok=false for any
// read or JSON error so a torn manifest never propagates as a hard failure.
func readSweepManifest(path string) (manifest sweepManifest, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sweepManifest{}, false
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return sweepManifest{}, false
	}
	return manifest, true
}

// removeManifests best-effort deletes each dead manifest once its admin
// dirs have been matched against the current lock scan.
func removeManifests(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

// readWorktreeAdminDir resolves a worktree's admin dir from its `.git`
// pointer file (no git subprocess); ok=false when missing or malformed.
func readWorktreeAdminDir(worktreePath string) (adminDir string, ok bool) {
	data, err := os.ReadFile(filepath.Join(worktreePath, ".git"))
	if err != nil {
		return "", false
	}
	dir, found := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !found {
		return "", false
	}
	return strings.TrimSpace(dir), true
}

// atomicWriteFile writes data to a temp file in dir then renames it to
// path, so a reader never observes a partially-written manifest.
func atomicWriteFile(dir, path string, data []byte) error {
	tmp, err := os.CreateTemp(dir, "sweep-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path) //nolint:gosec // path is built from commonDir + this process's own PID, not user input
}
