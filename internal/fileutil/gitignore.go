package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	gitignoreLockDirName   = ".gitignore.lock"
	gitignoreLockOwnerFile = "owner"
)

var (
	gitignoreLockTimeout  atomic.Int64
	gitignoreStaleLockAge atomic.Int64
)

func init() {
	gitignoreLockTimeout.Store(int64(2 * time.Second))
	gitignoreStaleLockAge.Store(int64(5 * time.Minute))
}

// EnsureGitignore ensures that entry is present as a line in the .gitignore
// file at repoRoot. If the file does not exist it is created. Returns true
// if the entry was added, false if it was already present.
func EnsureGitignore(repoRoot string, entry string) (added bool, retErr error) {
	return withGitignoreLock(repoRoot, func() (bool, error) {
		return ensureGitignoreLocked(repoRoot, entry)
	})
}

// EnsureLocalGlobIgnored consolidates *.local.toml overrides under a single
// .rimba/*.local.toml gitignore glob, removing any pre-existing per-file entries.
// No-op when .rimba/ is already ignored (--personal repos).
// Returns whether the glob line was newly added.
func EnsureLocalGlobIgnored(repoRoot string) (added bool, err error) {
	return withGitignoreLock(repoRoot, func() (bool, error) {
		hasDir, err := hasGitignoreEntry(repoRoot, config.DirName+"/")
		if err != nil || hasDir {
			return false, err
		}
		// Best-effort cleanup: the glob below covers both files even if removal fails.
		removeGitignoreEntryVariantsLocked(repoRoot, config.DirName, config.LocalFile)
		removeGitignoreEntryVariantsLocked(repoRoot, config.DirName, config.TrustFile)
		return ensureGitignoreLocked(repoRoot, config.DirName+"/"+config.LocalGlob)
	})
}

// RemoveGitignoreEntry removes entry from the .gitignore file at repoRoot.
// Returns true if the entry was removed, false if the file doesn't exist or
// the entry was not present.
func RemoveGitignoreEntry(repoRoot string, entry string) (bool, error) {
	return withGitignoreLock(repoRoot, func() (bool, error) {
		return removeGitignoreEntryLocked(repoRoot, entry)
	})
}

func ensureGitignoreLocked(repoRoot string, entry string) (added bool, retErr error) {
	path := filepath.Join(repoRoot, ".gitignore")

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	content := string(data)

	// Check whether entry already exists as a line.
	for line := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return false, nil
		}
	}

	// Build the line to append.
	var buf strings.Builder
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		buf.WriteByte('\n')
	}
	buf.WriteString(entry)
	buf.WriteByte('\n')

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644) //nolint:gosec // .gitignore must be world-readable for git
	if err != nil {
		return false, err
	}
	defer func() {
		if cerr := f.Close(); retErr == nil {
			retErr = cerr
		}
	}()

	if _, err := f.WriteString(buf.String()); err != nil {
		return false, err
	}

	return true, nil
}

// hasGitignoreEntry reports whether entry is present as a trimmed line in the
// .gitignore file at repoRoot. Returns false (not error) when the file is absent.
//
// Not lock-guarded: pairing this with a later Ensure/Remove call re-opens the
// TOCTOU window those functions close. Only call it as a standalone read, and
// take the result as advisory the instant a concurrent writer might run.
func hasGitignoreEntry(repoRoot, entry string) (bool, error) {
	path := filepath.Join(repoRoot, ".gitignore")
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return true, nil
		}
	}
	return false, nil
}

func removeGitignoreEntryLocked(repoRoot string, entry string) (bool, error) {
	path := filepath.Join(repoRoot, ".gitignore")

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	lines := strings.Split(string(data), "\n")
	var filtered []string
	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			found = true
			continue
		}
		filtered = append(filtered, line)
	}

	if !found {
		return false, nil
	}

	return true, os.WriteFile(path, []byte(strings.Join(filtered, "\n")), 0644) //nolint:gosec // .gitignore must be world-readable for git
}

func withGitignoreLock(repoRoot string, fn func() (bool, error)) (retAdded bool, retErr error) {
	lockPath, err := ensureGitignoreLockDir(repoRoot)
	if err != nil {
		return false, err
	}

	unlock, err := acquireGitignoreLock(lockPath)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := unlock(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	return fn()
}

// ensureGitignoreLockDir makes sure repoRoot/.rimba exists and returns the
// path of the lock directory beneath it. repoRoot itself must already exist;
// a missing repoRoot surfaces as an error here rather than being silently
// created.
func ensureGitignoreLockDir(repoRoot string) (string, error) {
	lockDir := filepath.Join(repoRoot, config.DirName)
	if err := os.Mkdir(lockDir, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}
	return filepath.Join(lockDir, gitignoreLockDirName), nil
}

func acquireGitignoreLock(lockPath string) (func() error, error) {
	deadline := time.Now().Add(time.Duration(gitignoreLockTimeout.Load()))
	for {
		if err := os.Mkdir(lockPath, 0700); err == nil {
			writeGitignoreLockOwner(lockPath)
			return func() error {
				return os.RemoveAll(lockPath)
			}, nil
		} else if !os.IsExist(err) {
			return nil, err
		}

		if reclaimStaleGitignoreLock(lockPath) {
			continue
		}

		if time.Now().After(deadline) {
			return nil, errors.New("timed out waiting for .gitignore lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// reclaimStaleGitignoreLock removes lockPath when it looks abandoned: either
// its owning process is provably dead (Unix liveness check) or it has aged
// past gitignoreStaleLockAge. The age fallback is what recovers orphaned
// locks on Windows and covers PID reuse everywhere. A failed reclaim (e.g.
// another process already cleaned it up) is reported as "not reclaimed" so
// the caller just spins and retries — it is never treated as an error.
func reclaimStaleGitignoreLock(lockPath string) bool {
	info, err := os.Stat(lockPath)
	if err != nil {
		return false
	}

	stale := time.Since(info.ModTime()) > time.Duration(gitignoreStaleLockAge.Load())
	if !stale {
		if pid, ok := readGitignoreLockOwner(lockPath); ok && !gitignoreLockOwnerAlive(pid) {
			stale = true
		}
	}
	if !stale {
		return false
	}

	return os.RemoveAll(lockPath) == nil
}

// writeGitignoreLockOwner records the acquiring process's PID inside the
// lock directory so a later holder can attempt a liveness-based reclaim.
// Best-effort: a failed write just leaves reclaim to the age fallback.
func writeGitignoreLockOwner(lockPath string) {
	_ = os.WriteFile(filepath.Join(lockPath, gitignoreLockOwnerFile), []byte(strconv.Itoa(os.Getpid())), 0600)
}

func readGitignoreLockOwner(lockPath string) (int, bool) {
	data, err := os.ReadFile(filepath.Join(lockPath, gitignoreLockOwnerFile))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func removeGitignoreEntryVariantsLocked(repoRoot, dir, file string) {
	_, _ = removeGitignoreEntryLocked(repoRoot, dir+"/"+file)
	// Pre-normalization .gitignore files may still carry the legacy
	// Windows-style backslash separator; migrate those too.
	_, _ = removeGitignoreEntryLocked(repoRoot, dir+"\\"+file)
}
