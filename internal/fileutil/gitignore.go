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
	// gitignoreLockDirName is actually a directory (an os.Mkdir-based lock),
	// not a TOML file. Named to match *.local.toml so the existing
	// .rimba/*.local.toml gitignore glob already hides it.
	gitignoreLockDirName   = "gitignore-lock.local.toml"
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
// Not lock-guarded: calling this and then a separate Ensure/Remove call
// re-opens the TOCTOU race those functions close. Use it as a standalone
// read only.
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
//
// Note: an older build still locking at the pre-relocation path
// (<repoRoot>/.gitignore.lock) won't be excluded by this lock.
func ensureGitignoreLockDir(repoRoot string) (string, error) {
	lockDir := filepath.Join(repoRoot, config.DirName)
	if err := os.Mkdir(lockDir, 0750); err != nil && !os.IsExist(err) {
		return "", err
	}
	return filepath.Join(lockDir, gitignoreLockDirName), nil
}

func acquireGitignoreLock(lockPath string) (func() error, error) {
	deadline := time.Now().Add(time.Duration(gitignoreLockTimeout.Load()))
	for {
		if err := os.Mkdir(lockPath, 0700); err == nil {
			token := writeGitignoreLockOwner(lockPath)
			return func() error {
				return releaseGitignoreLockIfOwned(lockPath, token)
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
// someone else already reclaimed it) is reported as "not reclaimed" so the
// caller just spins and retries — never treated as an error.
func reclaimStaleGitignoreLock(lockPath string) bool {
	info, err := os.Stat(lockPath)
	if err != nil {
		return false
	}

	pid, token, hasOwner := readGitignoreLockOwner(lockPath)
	stale := time.Since(info.ModTime()) > time.Duration(gitignoreStaleLockAge.Load())
	if !stale && hasOwner && !gitignoreLockOwnerAlive(pid) {
		stale = true
	}
	if !stale {
		return false
	}

	if !hasOwner {
		// No metadata to check against, likely a crash before it could be
		// written. Just remove it.
		return os.RemoveAll(lockPath) == nil
	}
	// Re-check the owner right before deleting: if it changed since we
	// read it above, someone else already released and re-acquired the
	// lock, so back off instead of deleting their active lock.
	if !gitignoreLockTokenMatches(lockPath, token) {
		return false
	}
	return os.RemoveAll(lockPath) == nil
}

// writeGitignoreLockOwner records this process's PID and a unique token in
// the lock directory, and returns that token so it can be checked later
// before deleting the directory. Best-effort: a failed write just leaves
// this one acquisition unfenced.
func writeGitignoreLockOwner(lockPath string) string {
	token := strconv.Itoa(os.Getpid()) + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
	_ = os.WriteFile(filepath.Join(lockPath, gitignoreLockOwnerFile), []byte(token), 0600)
	return token
}

// readGitignoreLockOwner parses the owner file written by
// writeGitignoreLockOwner, returning the recorded PID and the full token.
func readGitignoreLockOwner(lockPath string) (pid int, token string, ok bool) {
	data, err := os.ReadFile(filepath.Join(lockPath, gitignoreLockOwnerFile))
	if err != nil {
		return 0, "", false
	}
	token = strings.TrimSpace(string(data))
	pidPart, _, found := strings.Cut(token, ":")
	if !found {
		return 0, "", false
	}
	pid, err = strconv.Atoi(pidPart)
	if err != nil {
		return 0, "", false
	}
	return pid, token, true
}

func gitignoreLockTokenMatches(lockPath, token string) bool {
	_, current, ok := readGitignoreLockOwner(lockPath)
	return ok && current == token
}

// releaseGitignoreLockIfOwned removes lockPath only if its owner file still
// holds token — i.e. nobody has reclaimed the lock since this acquisition
// wrote it. A mismatch means a different process now owns this path, so
// this is a no-op rather than an error.
func releaseGitignoreLockIfOwned(lockPath, token string) error {
	if !gitignoreLockTokenMatches(lockPath, token) {
		return nil
	}
	return os.RemoveAll(lockPath)
}

func removeGitignoreEntryVariantsLocked(repoRoot, dir, file string) {
	_, _ = removeGitignoreEntryLocked(repoRoot, dir+"/"+file)
	// Pre-normalization .gitignore files may still carry the legacy
	// Windows-style backslash separator; migrate those too.
	_, _ = removeGitignoreEntryLocked(repoRoot, dir+"\\"+file)
}
