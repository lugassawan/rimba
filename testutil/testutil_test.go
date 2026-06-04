package testutil_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/testutil"
)

type fixtureTSpy struct {
	fatalfCalled bool
	fatalfMsg    string
}

func (s *fixtureTSpy) Helper() {}

func (s *fixtureTSpy) Fatalf(format string, args ...any) {
	s.fatalfCalled = true
	s.fatalfMsg = fmt.Sprintf(format, args...)
}

func TestPtr(t *testing.T) {
	p := testutil.Ptr(42)
	if p == nil || *p != 42 {
		t.Fatalf("Ptr(42) = %v, want &42", p)
	}
}

func TestLoadFixture(t *testing.T) {
	got := testutil.LoadFixture(t, "testdata/fixture.txt")
	if got != "hello fixture\n" {
		t.Fatalf("LoadFixture = %q, want %q", got, "hello fixture\n")
	}
}

func TestNewTestRepo(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
		t.Fatalf("expected .git dir in repo %s: %v", repo, err)
	}
}

func TestCreateFile(t *testing.T) {
	dir := t.TempDir()
	testutil.CreateFile(t, dir, "hello.txt", "world")
	data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "world" {
		t.Fatalf("content = %q, want %q", string(data), "world")
	}
}

func TestLoadFixtureMissingFile(t *testing.T) {
	spy := &fixtureTSpy{}
	got := testutil.LoadFixture(spy, "testdata/does-not-exist.txt")
	if !spy.fatalfCalled {
		t.Fatal("expected Fatalf to be called for missing fixture")
	}
	if got != "" {
		t.Fatalf("expected empty string on error, got %q", got)
	}
	if !strings.Contains(spy.fatalfMsg, "LoadFixture testdata/does-not-exist.txt") {
		t.Fatalf("expected message to contain %q, got %q", "LoadFixture testdata/does-not-exist.txt", spy.fatalfMsg)
	}
}

func TestGitCmd(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	out := testutil.GitCmd(t, repo, "log", "--oneline", "-1")
	if !strings.Contains(out, "initial commit") {
		t.Fatalf("git log = %q, want 'initial commit'", out)
	}
}
