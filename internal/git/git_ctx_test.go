package git

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lugassawan/rimba/testutil"
)

func TestRunInDirCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	r := &ExecRunner{}
	_, err := r.RunInDir(ctx, "", "status")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected 'context canceled' in error, got: %v", err)
	}
}

func TestRunDelegatesToRunInDir(t *testing.T) {
	r := &ExecRunner{}
	out, err := r.Run(context.Background(), "--version")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "git") {
		t.Errorf("expected git version output, got: %q", out)
	}
}

func TestExecRunnerTimeoutExpires(t *testing.T) {
	r := &ExecRunner{Timeout: time.Nanosecond}
	_, err := r.Run(context.Background(), "--version")
	if err == nil {
		t.Fatal("expected error from nanosecond timeout, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected 'context deadline exceeded', got: %v", err)
	}
}

func TestExecRunnerZeroTimeoutNoDeadline(t *testing.T) {
	r := &ExecRunner{Timeout: 0}
	out, err := r.Run(context.Background(), "--version")
	if err != nil {
		t.Fatalf("Run with zero timeout: %v", err)
	}
	if !strings.Contains(out, "git") {
		t.Errorf("expected git version output, got: %q", out)
	}
}

func TestStableGitEnvForcesCLocale(t *testing.T) {
	env := stableGitEnv([]string{
		"PATH=/usr/bin",
		"LANG=fr_FR.UTF-8",
		"LC_ALL=de_DE.UTF-8",
		"OTHER=value",
	})

	for _, entry := range env {
		if strings.HasPrefix(entry, "LANG=") && entry != "LANG=C" {
			t.Fatalf("unexpected LANG entry %q in %v", entry, env)
		}
		if strings.HasPrefix(entry, "LC_ALL=") && entry != "LC_ALL=C" {
			t.Fatalf("unexpected LC_ALL entry %q in %v", entry, env)
		}
	}

	for _, want := range []string{"PATH=/usr/bin", "OTHER=value", "LANG=C", "LC_ALL=C"} {
		if !hasEnv(env, want) {
			t.Fatalf("expected %q in %v", want, env)
		}
	}
}

func hasEnv(env []string, want string) bool {
	return slices.Contains(env, want)
}

func TestIsLongRunning(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"worktree_add", []string{"worktree", "add"}, true},
		{"worktree_remove", []string{"worktree", "remove"}, true},
		{"worktree_move", []string{"worktree", "move"}, true},
		{"worktree_list", []string{"worktree", "list"}, false},
		{"worktree_prune", []string{"worktree", "prune"}, false},
		{"non_worktree", []string{"status"}, false},
		{"empty_args", []string{}, false},
		{"single_arg_worktree_only", []string{"worktree"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLongRunning(tt.args); got != tt.want {
				t.Errorf("isLongRunning(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestRunInDirExemptsLongRunningFromTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip(skipIntegrationGit)
	}

	repo := testutil.NewTestRepo(t)
	r := &ExecRunner{Timeout: time.Nanosecond}

	wtPath := filepath.Join(filepath.Dir(repo), "wt-exempt-timeout")
	if _, err := r.RunInDir(context.Background(), repo, "worktree", "add", "-b", "exempt-timeout", "--", wtPath, "main"); err != nil {
		t.Fatalf("expected worktree add to bypass the tiny Timeout (long-running exemption), got: %v", err)
	}
}

// TestRunInDirGracefulShutdownSendsSIGTERM proves cancellation sends SIGTERM
// (letting git's own atexit cleanup run) rather than an uncatchable SIGKILL.
// A fake "git" shim on PATH traps TERM and leaves a sentinel file; if the real
// production code silently reverted to SIGKILL, no trap would fire and no
// sentinel would appear.
func TestRunInDirGracefulShutdownSendsSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signal semantics only")
	}

	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	sentinel := filepath.Join(dir, "sigterm-received")
	script := filepath.Join(dir, "git")
	body := "#!/bin/sh\n" +
		"trap 'touch \"$SENTINEL_PATH\"; exit 0' TERM\n" +
		"touch \"$STARTED_PATH\"\n" +
		"sleep 30 >/dev/null 2>&1 &\n" +
		"wait\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake git shim: %v", err)
	}

	t.Setenv("STARTED_PATH", started)
	t.Setenv("SENTINEL_PATH", sentinel)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	r := &ExecRunner{}
	done := make(chan struct{})
	go func() {
		_, _ = r.RunInDir(ctx, "", "status")
		close(done)
	}()

	// Poll for the shim's own "started" marker instead of guessing a fixed
	// sleep: a busy scheduler can delay the goroutine past any fixed delay,
	// which would cancel ctx before the subprocess ever starts.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake git shim never started")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("RunInDir did not return within 8s of cancellation; process may be stuck waiting for the WaitDelay/SIGKILL backstop")
	}

	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("expected SIGTERM sentinel file; fake git may have been SIGKILLed instead: %v", err)
	}
}
