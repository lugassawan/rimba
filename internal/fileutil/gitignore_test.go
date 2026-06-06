package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	testEntry      = ".rimba.toml"
	gitignoreFile  = ".gitignore"
	errUnexpected  = "unexpected error: %v"
	otherEntry     = "node_modules"
	otherEntryLine = otherEntry + "\n"
)

func TestEnsureGitignoreNoFile(t *testing.T) {
	dir := t.TempDir()

	added, err := EnsureGitignore(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !added {
		t.Fatal("expected added=true when .gitignore does not exist")
	}

	got := readFile(t, filepath.Join(dir, gitignoreFile))
	if got != testEntry+"\n" {
		t.Errorf("expected %q, got %q", testEntry+"\n", got)
	}
}

func TestEnsureGitignoreExistsWithoutEntry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), otherEntryLine)

	added, err := EnsureGitignore(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !added {
		t.Fatal("expected added=true")
	}

	got := readFile(t, filepath.Join(dir, gitignoreFile))
	if got != otherEntryLine+testEntry+"\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestEnsureGitignoreAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), otherEntryLine+testEntry+"\n")

	added, err := EnsureGitignore(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if added {
		t.Fatal("expected added=false when entry already present")
	}

	got := readFile(t, filepath.Join(dir, gitignoreFile))
	if got != otherEntryLine+testEntry+"\n" {
		t.Errorf("content should be unchanged, got %q", got)
	}
}

func TestEnsureGitignoreNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), otherEntry)

	added, err := EnsureGitignore(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !added {
		t.Fatal("expected added=true")
	}

	got := readFile(t, filepath.Join(dir, gitignoreFile))
	if !strings.HasSuffix(got, "\n"+testEntry+"\n") {
		t.Errorf("expected entry on its own line, got %q", got)
	}
}

func TestEnsureGitignoreWhitespaceMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), "  "+testEntry+"  \n")

	added, err := EnsureGitignore(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if added {
		t.Fatal("expected added=false when entry present with surrounding whitespace")
	}
}

func TestEnsureGitignoreIdempotent(t *testing.T) {
	dir := t.TempDir()

	for range 3 {
		if _, err := EnsureGitignore(dir, testEntry); err != nil {
			t.Fatalf(errUnexpected, err)
		}
	}

	got := readFile(t, filepath.Join(dir, gitignoreFile))
	if strings.Count(got, testEntry) != 1 {
		t.Errorf("expected exactly one occurrence, got %q", got)
	}
}

func TestEnsureGitignoreConcurrent(t *testing.T) {
	dir := t.TempDir()

	const workers = 8
	previousTimeout := gitignoreLockTimeout
	gitignoreLockTimeout = 10 * time.Second
	t.Cleanup(func() {
		gitignoreLockTimeout = previousTimeout
	})

	start := make(chan struct{})
	addedResults := make(chan bool, workers)
	errs := make(chan error, workers)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			<-start
			added, err := EnsureGitignore(dir, testEntry)
			if err != nil {
				errs <- err
				return
			}
			addedResults <- added
		})
	}

	close(start)
	wg.Wait()
	close(errs)
	close(addedResults)

	for err := range errs {
		if err != nil {
			t.Fatalf(errUnexpected, err)
		}
	}

	addedCount := 0
	for added := range addedResults {
		if added {
			addedCount++
		}
	}
	if addedCount != 1 {
		t.Fatalf("expected exactly one caller to add the entry, got %d", addedCount)
	}

	got := readFile(t, filepath.Join(dir, gitignoreFile))
	if strings.Count(got, testEntry) != 1 {
		t.Errorf("expected exactly one occurrence, got %q", got)
	}
}

func TestEnsureGitignoreLockParentMissing(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "missing")

	added, err := EnsureGitignore(repoRoot, testEntry)
	if err == nil {
		t.Fatal("expected error when lock parent is missing")
	}
	if added {
		t.Fatal("expected added=false when lock cannot be acquired")
	}
}

func TestEnsureGitignoreLockTimeout(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".gitignore.lock")
	if err := os.Mkdir(lockPath, 0700); err != nil {
		t.Fatal(err)
	}

	previousTimeout := gitignoreLockTimeout
	gitignoreLockTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		gitignoreLockTimeout = previousTimeout
		_ = os.Remove(lockPath)
	})

	added, err := EnsureGitignore(dir, testEntry)
	if err == nil {
		t.Fatal("expected error when .gitignore lock is held")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if added {
		t.Fatal("expected added=false when lock cannot be acquired")
	}
}

func TestWithGitignoreLockUnlockError(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".gitignore.lock")
	t.Cleanup(func() { _ = os.RemoveAll(lockPath) })

	added, err := withGitignoreLock(dir, func() (bool, error) {
		if err := os.WriteFile(filepath.Join(lockPath, "child"), []byte("held"), 0644); err != nil {
			return false, err
		}
		return true, nil
	})
	if err == nil {
		t.Fatal("expected error when lock directory cannot be removed")
	}
	if !added {
		t.Fatal("expected added result from callback to be preserved")
	}
}

func TestEnsureGitignoreReadError(t *testing.T) {
	dir := t.TempDir()
	// Create .gitignore as a directory so ReadFile returns non-IsNotExist error
	if err := os.Mkdir(filepath.Join(dir, gitignoreFile), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := EnsureGitignore(dir, testEntry)
	if err == nil {
		t.Fatal("expected error when .gitignore is a directory")
	}
}

func TestEnsureGitignoreOpenError(t *testing.T) {
	dir := t.TempDir()

	// Make directory read-only so OpenFile fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
	probePath := filepath.Join(dir, ".write-probe")
	if err := os.WriteFile(probePath, []byte("probe"), 0644); err == nil {
		_ = os.Remove(probePath)
		t.Skip("directory permissions do not block writes on this platform")
	}

	_, err := EnsureGitignore(dir, testEntry)
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
}

func TestRemoveGitignoreEntryPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), otherEntryLine+testEntry+"\n")

	removed, err := RemoveGitignoreEntry(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !removed {
		t.Fatal("expected removed=true when entry is present")
	}

	got := readFile(t, filepath.Join(dir, gitignoreFile))
	if strings.Contains(got, testEntry) {
		t.Errorf("expected entry to be removed, got %q", got)
	}
	if !strings.Contains(got, otherEntry) {
		t.Errorf("expected other entries to be preserved, got %q", got)
	}
}

func TestRemoveGitignoreEntryNotPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), otherEntryLine)

	removed, err := RemoveGitignoreEntry(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if removed {
		t.Fatal("expected removed=false when entry not present")
	}
}

func TestRemoveGitignoreEntryNoFile(t *testing.T) {
	dir := t.TempDir()

	removed, err := RemoveGitignoreEntry(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if removed {
		t.Fatal("expected removed=false when .gitignore does not exist")
	}
}

func TestRemoveGitignoreEntryReadError(t *testing.T) {
	dir := t.TempDir()
	// Create .gitignore as a directory so ReadFile returns non-IsNotExist error
	if err := os.Mkdir(filepath.Join(dir, gitignoreFile), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := RemoveGitignoreEntry(dir, testEntry)
	if err == nil {
		t.Fatal("expected error when .gitignore is a directory")
	}
}

func TestRemoveGitignoreEntryWriteError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, gitignoreFile)
	writeFile(t, path, testEntry+"\n")

	// Make the file itself read-only so WriteFile fails
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })

	_, err := RemoveGitignoreEntry(dir, testEntry)
	if err == nil {
		t.Fatal("expected error when .gitignore is read-only")
	}
}

func TestHasGitignoreEntryNoFile(t *testing.T) {
	dir := t.TempDir()

	present, err := HasGitignoreEntry(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if present {
		t.Fatal("expected present=false when .gitignore does not exist")
	}
}

func TestHasGitignoreEntryPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), otherEntryLine+testEntry+"\n")

	present, err := HasGitignoreEntry(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !present {
		t.Fatal("expected present=true when entry is in .gitignore")
	}
}

func TestHasGitignoreEntryNotPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile), otherEntryLine)

	present, err := HasGitignoreEntry(dir, testEntry)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if present {
		t.Fatal("expected present=false when entry is not in .gitignore")
	}
}

func TestHasGitignoreEntryReadError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, gitignoreFile), 0755); err != nil {
		t.Fatal(err)
	}

	present, err := HasGitignoreEntry(dir, testEntry)
	if err == nil {
		t.Fatal("expected error when .gitignore is a directory")
	}
	if present {
		t.Fatal("expected present=false on read error")
	}
}

func TestEnsureLocalGlobIgnoredPersonalMode(t *testing.T) {
	dir := t.TempDir()
	original := ".rimba/\n"
	writeFile(t, filepath.Join(dir, gitignoreFile), original)

	added, err := EnsureLocalGlobIgnored(dir)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if added {
		t.Error("expected added=false in personal mode (.rimba/ already ignored)")
	}
	if got := readFile(t, filepath.Join(dir, gitignoreFile)); got != original {
		t.Errorf(".gitignore should be unchanged in personal mode, got:\n%s", got)
	}
}

func TestEnsureLocalGlobIgnoredMigration(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile),
		"node_modules\n.rimba/settings.local.toml\n.rimba/trust.local.toml\n")

	added, err := EnsureLocalGlobIgnored(dir)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !added {
		t.Error("expected added=true when glob was not yet present")
	}

	content := readFile(t, filepath.Join(dir, gitignoreFile))
	glob := config.DirName + "/" + config.LocalGlob
	if !strings.Contains(content, glob) {
		t.Errorf(".gitignore should contain %q, got:\n%s", glob, content)
	}
	if strings.Contains(content, ".rimba/settings.local.toml") {
		t.Errorf(".gitignore should not contain per-file settings entry, got:\n%s", content)
	}
	if strings.Contains(content, ".rimba/trust.local.toml") {
		t.Errorf(".gitignore should not contain per-file trust entry, got:\n%s", content)
	}
	if !strings.Contains(content, "node_modules") {
		t.Errorf(".gitignore should preserve other entries, got:\n%s", content)
	}
}

func TestEnsureLocalGlobIgnoredMigratesLegacyBackslashEntries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, gitignoreFile),
		"node_modules\n.rimba\\settings.local.toml\n.rimba\\trust.local.toml\n")

	added, err := EnsureLocalGlobIgnored(dir)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !added {
		t.Error("expected added=true when glob was not yet present")
	}

	content := readFile(t, filepath.Join(dir, gitignoreFile))
	glob := config.DirName + "/" + config.LocalGlob
	if !strings.Contains(content, glob) {
		t.Errorf(".gitignore should contain %q, got:\n%s", glob, content)
	}
	if strings.Contains(content, ".rimba\\settings.local.toml") {
		t.Errorf(".gitignore should not contain legacy settings entry, got:\n%s", content)
	}
	if strings.Contains(content, ".rimba\\trust.local.toml") {
		t.Errorf(".gitignore should not contain legacy trust entry, got:\n%s", content)
	}
	if !strings.Contains(content, "node_modules") {
		t.Errorf(".gitignore should preserve other entries, got:\n%s", content)
	}
}

func TestEnsureLocalGlobIgnoredNoGitignore(t *testing.T) {
	dir := t.TempDir()

	added, err := EnsureLocalGlobIgnored(dir)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}
	if !added {
		t.Error("expected added=true for fresh repo with no .gitignore")
	}
	content := readFile(t, filepath.Join(dir, gitignoreFile))
	glob := config.DirName + "/" + config.LocalGlob
	if !strings.Contains(content, glob) {
		t.Errorf("expected glob %q in new .gitignore, got:\n%s", glob, content)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
