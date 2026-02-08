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
