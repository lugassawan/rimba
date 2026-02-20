package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
