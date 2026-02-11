package cmd

import (
	"strings"
	"testing"
)

func TestOpenPrintPath(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		wtFeatureLogin,
		"HEAD def456",
		"branch refs/heads/feature/login",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	openCmd.SetOut(buf)
	openCmd.SetErr(buf)
	openCmd.SetArgs([]string{"login"})
	if err := openCmd.RunE(cmd, []string{"login"}); err != nil {
		t.Fatalf("openCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "/wt/feature-login") {
		t.Errorf("output = %q, want path containing '/wt/feature-login'", out)
	}
}

func TestOpenWorktreeNotFound(t *testing.T) {
	porcelain := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	err := openCmd.RunE(cmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent worktree")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("error = %q, want 'worktree not found'", err.Error())
	}
}
