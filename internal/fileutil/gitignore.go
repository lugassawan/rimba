package fileutil

import (
	"os"
	"path/filepath"
	"strings"
)

// EnsureGitignore ensures that entry is present as a line in the .gitignore
// file at repoRoot. If the file does not exist it is created. Returns true
// if the entry was added, false if it was already present.
func EnsureGitignore(repoRoot string, entry string) (added bool, retErr error) {
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

// RemoveGitignoreEntry removes entry from the .gitignore file at repoRoot.
// Returns true if the entry was removed, false if the file doesn't exist or
// the entry was not present.
func RemoveGitignoreEntry(repoRoot string, entry string) (bool, error) {
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
