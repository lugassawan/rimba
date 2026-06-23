package git

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
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
		t.Errorf("expected 'context deadline exceeded', got: %v", err)
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
