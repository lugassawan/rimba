package git

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestRunInDirContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	r := &ExecRunner{}
	_, err := r.RunInDirContext(ctx, "", "status")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected 'context canceled' in error, got: %v", err)
	}
}

func TestRunDelegatesToContext(t *testing.T) {
	r := &ExecRunner{}
	// git --version is a fast, safe command available everywhere
	out, err := r.Run("--version")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "git") {
		t.Errorf("expected git version output, got: %q", out)
	}
}

func TestRunContextDelegatesToRunInDirContext(t *testing.T) {
	r := &ExecRunner{}
	out, err := r.RunContext(context.Background(), "--version")
	if err != nil {
		t.Fatalf("RunContext: %v", err)
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
