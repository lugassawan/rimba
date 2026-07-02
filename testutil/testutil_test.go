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
	tempDir      string
}

func (s *fixtureTSpy) Helper() {}

func (s *fixtureTSpy) Fatalf(format string, args ...any) {
	s.fatalfCalled = true
	s.fatalfMsg = fmt.Sprintf(format, args...)
}

func (s *fixtureTSpy) TempDir() string {
	return s.tempDir
}

func assertFatalContains(t *testing.T, spy *fixtureTSpy, want string) {
	t.Helper()
	if !spy.fatalfCalled {
		t.Fatal("expected Fatalf to be called")
	}
	if !strings.Contains(spy.fatalfMsg, want) {
		t.Fatalf("expected message to contain %q, got %q", want, spy.fatalfMsg)
	}
}

func TestPtr(t *testing.T) {
	p := testutil.Ptr(42)
	if p == nil || *p != 42 {
		t.Fatalf("Ptr(42) = %v, want &42", p)
	}
}

func TestLoadFixture(t *testing.T) {
	got := strings.ReplaceAll(testutil.LoadFixture(t, "testdata/fixture.txt"), "\r\n", "\n")
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

func TestNewTestRepoFatalBranches(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) *fixtureTSpy
		wantMsg string
	}{
		{
			name: "mkdir failure",
			setup: func(t *testing.T) *fixtureTSpy {
				t.Helper()
				blocked := filepath.Join(t.TempDir(), "blocked")
				if err := os.WriteFile(blocked, []byte("file"), 0644); err != nil {
					t.Fatalf("write blocker: %v", err)
				}
				return &fixtureTSpy{tempDir: blocked}
			},
			wantMsg: "mkdir:",
		},
		{
			name: "git init failure",
			setup: func(t *testing.T) *fixtureTSpy {
				t.Helper()
				t.Setenv("PATH", "")
				return &fixtureTSpy{tempDir: t.TempDir()}
			},
			wantMsg: "cmd [git init -b main]",
		},
		{
			name: "readme write failure",
			setup: func(t *testing.T) *fixtureTSpy {
				t.Helper()
				dir := t.TempDir()
				if err := os.MkdirAll(filepath.Join(dir, "test-repo", "README.md"), 0755); err != nil {
					t.Fatalf("mkdir readme blocker: %v", err)
				}
				return &fixtureTSpy{tempDir: dir}
			},
			wantMsg: "write:",
		},
		{
			name: "git add failure",
			setup: func(t *testing.T) *fixtureTSpy {
				t.Helper()
				indexDir := filepath.Join(t.TempDir(), "index")
				if err := os.MkdirAll(indexDir, 0755); err != nil {
					t.Fatalf("mkdir index blocker: %v", err)
				}
				t.Setenv("GIT_INDEX_FILE", indexDir)
				return &fixtureTSpy{tempDir: t.TempDir()}
			},
			wantMsg: "cmd [git add .]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			spy := tt.setup(t)
			if got := testutil.NewTestRepo(spy); got != "" {
				t.Fatalf("NewTestRepo returned %q, want empty path on failure", got)
			}
			assertFatalContains(t, spy, tt.wantMsg)
		})
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

func TestCreateFileFailure(t *testing.T) {
	spy := &fixtureTSpy{}
	testutil.CreateFile(spy, t.TempDir(), filepath.Join("missing", "hello.txt"), "world")
	assertFatalContains(t, spy, "write ")
}

func TestLoadFixtureMissingFile(t *testing.T) {
	spy := &fixtureTSpy{}
	got := testutil.LoadFixture(spy, "testdata/does-not-exist.txt")
	if got != "" {
		t.Fatalf("expected empty string on error, got %q", got)
	}
	assertFatalContains(t, spy, "LoadFixture testdata/does-not-exist.txt")
}

func TestGitCmd(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	out := testutil.GitCmd(t, repo, "log", "--oneline", "-1")
	if !strings.Contains(out, "initial commit") {
		t.Fatalf("git log = %q, want 'initial commit'", out)
	}
}

func TestGitCmdFailure(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	spy := &fixtureTSpy{}
	got := testutil.GitCmd(spy, repo, "not-a-real-subcommand")
	if got != "" {
		t.Fatalf("GitCmd returned %q, want empty output on failure", got)
	}
	assertFatalContains(t, spy, "git [not-a-real-subcommand]")
}
