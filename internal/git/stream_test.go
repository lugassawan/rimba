package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFakeGit puts a fake "git" executable at the front of PATH so tests can
// control stdout/stderr/exit-code deterministically, independent of the
// installed git version's actual message wording.
func writeFakeGit(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake git shim requires a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "git")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunInDirSuccessDoesNotLeakStderrIntoResult(t *testing.T) {
	writeFakeGit(t, "#!/bin/sh\necho 'warning: noisy' 1>&2\necho 'clean-output'\nexit 0\n")

	r := &ExecRunner{}
	out, err := r.Run(context.Background(), "status")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "clean-output" {
		t.Errorf("result = %q, want %q", out, "clean-output")
	}
}

func TestRunInDirErrorIncludesStderr(t *testing.T) {
	writeFakeGit(t, "#!/bin/sh\necho 'ignored stdout'\necho 'fatal: boom' 1>&2\nexit 1\n")

	r := &ExecRunner{}
	_, err := r.Run(context.Background(), "status")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fatal: boom") {
		t.Errorf("error = %v, want it to contain stderr text", err)
	}
}

func TestRunInDirErrorFallsBackToStdoutWhenStderrEmpty(t *testing.T) {
	writeFakeGit(t, "#!/bin/sh\necho 'only stdout message'\nexit 1\n")

	r := &ExecRunner{}
	_, err := r.Run(context.Background(), "status")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "only stdout message") {
		t.Errorf("error = %v, want fallback to stdout text", err)
	}
}
