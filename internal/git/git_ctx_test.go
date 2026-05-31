package git

import (
	"context"
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
