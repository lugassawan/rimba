package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/config"
)

var gitignoreLockTimeout = 2 * time.Second

// EnsureGitignore ensures that entry is present as a line in the .gitignore
// file at repoRoot. If the file does not exist it is created. Returns true
// if the entry was added, false if it was already present.
func EnsureGitignore(repoRoot string, entry string) (added bool, retErr error) {
	return withGitignoreLock(repoRoot, func() (bool, error) {
		return ensureGitignoreLocked(repoRoot, entry)
	})
}

// HasGitignoreEntry reports whether entry is present as a trimmed line in the
// .gitignore file at repoRoot. Returns false (not error) when the file is absent.
func HasGitignoreEntry(repoRoot, entry string) (bool, error) {
	return hasGitignoreEntry(repoRoot, entry)
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
		return ensureGitignoreLocked(repoRoot, gitignorePattern(config.DirName, config.LocalGlob))
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
	lockPath := filepath.Join(repoRoot, ".gitignore.lock")
	unlock, err := acquireGitignoreLock(lockPath)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := unlock(); retErr == nil && err != nil && !os.IsNotExist(err) {
			retErr = err
		}
	}()

	return fn()
}

func acquireGitignoreLock(lockPath string) (func() error, error) {
	deadline := time.Now().Add(gitignoreLockTimeout)
	for {
		if err := os.Mkdir(lockPath, 0700); err == nil {
			return func() error {
				return os.Remove(lockPath)
			}, nil
		} else if !os.IsExist(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, errors.New("timed out waiting for .gitignore lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func gitignorePattern(dir, file string) string {
	return dir + "/" + file
}

func removeGitignoreEntryVariantsLocked(repoRoot, dir, file string) {
	entry := gitignorePattern(dir, file)
	_, _ = removeGitignoreEntryLocked(repoRoot, entry)
	if legacyEntry := dir + "\\" + file; legacyEntry != entry {
		_, _ = removeGitignoreEntryLocked(repoRoot, legacyEntry)
	}
}
