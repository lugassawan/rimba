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

// aliveMarkerCeiling bounds how long an "alive" classification is trusted,
// mirroring internal/fileutil/gitignore.go's gitignoreStaleLockAge. proc.Alive
// always reports true on Windows (no signal-0 probe there), and a PID can be
// reused by an unrelated process, so a manifest still "alive" past this
// ceiling is downgraded to markerless (neither confidently alive nor dead)
// instead of permanently vetoing the manual, age-based doctor --fix path.
const aliveMarkerCeiling = 5 * time.Minute

// sweepManifest records enough about an in-flight clean/remove/merge sweep
// to let a later run prove — rather than guess — that a stale index.lock
// belongs to a sweep whose owning process is confirmed dead (#383).
type sweepManifest struct {
	PID           int      `json:"pid"`
	StartUnixNano int64    `json:"start_unix_nano"`
	AdminDirs     []string `json:"admin_dirs"`
}

// ReapConfidentLocks removes index.lock files that can be attributed to a
// dead sweep with confidence: the lock's worktree admin dir appears in a
// sweep manifest whose owning PID is confirmed dead, and the lock is at
// least MinLockAge old (guards against a fresh sweep racing this scan).
// Manifests with a markerless lock, an alive owner, or a torn/unparseable
// body are left untouched — they stay on the manual, age-based `doctor
// --fix` path instead.
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

// AliveSweepAdminDirs returns the admin dirs claimed by sweep manifests
// whose owning PID is confirmed alive. `rimba doctor` uses this to
// distinguish a marker+alive lock (still owned by a running sweep, so it
// must never be touched even under --force) from a markerless one that
// falls back to the age-based removal flow.
func AliveSweepAdminDirs(commonDir string) map[string]bool {
	_, aliveAdminDirs, _ := classifySweepManifests(commonDir)
	return aliveAdminDirs
}

// writeSweepManifest records this process's PID, start time, and the admin
// dirs of worktreePaths, so a later confident reap can attribute an
// orphaned lock back to this sweep. Candidates whose `.git` pointer file
// can't be read (e.g. a prunable worktree, #374) are skipped — there's no
// admin dir to resolve. The returned cleanup is always safe to call and
// best-effort: a write failure here must never abort the actual removal.
func writeSweepManifest(commonDir string, worktreePaths []string) (cleanup func()) {
	noop := func() {}

	adminDirs := make([]string, 0, len(worktreePaths))
	for _, p := range worktreePaths {
		if dir, ok := readWorktreeAdminDir(p); ok {
			adminDirs = append(adminDirs, dir)
		}
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

// deferSweepManifest resolves commonDir and writes a sweep manifest for
// paths, in one call so the result can be deferred at the call site:
// defer deferSweepManifest(ctx, r, paths)(). Best-effort: a CommonDir
// failure, or a non-absolute result (seen in loosely-stubbed test runners,
// which must never resolve to the process's real working directory),
// skips the manifest rather than aborting the caller.
func deferSweepManifest(ctx context.Context, r git.Runner, paths []string) func() {
	commonDir, err := git.CommonDir(ctx, r)
	if err != nil || !filepath.IsAbs(commonDir) {
		return func() {}
	}
	return writeSweepManifest(commonDir, paths)
}

// classifySweepManifests scans every sweep manifest under commonDir and
// splits their recorded admin dirs by whether the owning PID is confirmed
// dead or alive; deadManifests collects the paths of dead-owner manifests
// so a caller can remove them once locks have been matched against
// deadDirs. Torn/unparseable manifests are skipped (fail-soft); an
// alive-owner manifest is left in place for a later run to re-check, unless
// it's past aliveMarkerCeiling, in which case it's dropped from both sets
// (neither confidently alive nor dead) rather than trusted indefinitely. A
// dead-owner admin dir is excluded if the directory itself was recreated
// after the manifest was written — a worktree basename reused by a later,
// unrelated worktree, whose own lock must not be attributed to this manifest.
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

// classifyManifest routes one manifest's admin dirs into deadDirs or
// aliveDirs based on proc.Alive, the alive-marker ceiling (Windows/PID-reuse
// guard), and — for a dead owner — the admin-dir-recreation guard
// (worktree-basename-reuse guard). Reports whether the owner is confidently
// dead, so the caller can queue this manifest for removal.
func classifyManifest(manifest sweepManifest, deadDirs, aliveDirs map[string]bool) (dead bool) {
	if proc.Alive(manifest.PID) {
		if !manifestPastCeiling(manifest) {
			for _, dir := range manifest.AdminDirs {
				aliveDirs[dir] = true
			}
		}
		return false
	}

	for _, dir := range manifest.AdminDirs {
		if !adminDirRecreatedSince(dir, manifest.StartUnixNano) {
			deadDirs[dir] = true
		}
	}
	return true
}

// manifestPastCeiling reports whether manifest was written more than
// aliveMarkerCeiling ago.
func manifestPastCeiling(manifest sweepManifest) bool {
	return time.Since(time.Unix(0, manifest.StartUnixNano)) > aliveMarkerCeiling
}

// adminDirRecreatedSince reports whether adminDir's directory entry was
// (re)created after manifestStart — i.e. a worktree basename was reused
// after the manifest's sweep started, so its recorded PID no longer
// attests to whatever now lives at that path. A missing adminDir is not a
// match (there's no lock to reap there either way).
func adminDirRecreatedSince(adminDir string, manifestStart int64) bool {
	info, err := os.Stat(adminDir)
	if err != nil {
		return false
	}
	return info.ModTime().UnixNano() > manifestStart
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

// readWorktreeAdminDir resolves a worktree's admin dir by reading its `.git`
// pointer file (`gitdir: <commonDir>/worktrees/<id>`) — a plain file read,
// no git subprocess. Returns ok=false when the file is missing or malformed
// (e.g. a prunable worktree whose `.git` is already gone, #374).
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
