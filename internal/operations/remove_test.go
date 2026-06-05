package operations

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

func TestRemoveWorktreeSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			if strings.Contains(cmd, "worktree remove") {
				return "", nil
			}
			if strings.Contains(cmd, "branch -D") {
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login"}
	result, err := RemoveWorktree(r, wt, "login", false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved to be true")
	}
	if !result.BranchDeleted {
		t.Error("expected BranchDeleted to be true")
	}
	if result.BranchError != nil {
		t.Errorf("unexpected BranchError: %v", result.BranchError)
	}
	if result.Task != "login" {
		t.Errorf("expected task %q, got %q", "login", result.Task)
	}
}

func TestRemoveWorktreeKeepBranch(t *testing.T) {
	branchDeleted := false
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			if strings.Contains(cmd, "worktree remove") {
				return "", nil
			}
			if strings.Contains(cmd, "branch -D") {
				branchDeleted = true
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login"}
	result, err := RemoveWorktree(r, wt, "login", true, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved to be true")
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false")
	}
	if branchDeleted {
		t.Error("branch should not have been deleted when keepBranch is true")
	}
}

func TestRemoveWorktreeRemovalFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("worktree is dirty")
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login"}
	result, err := RemoveWorktree(r, wt, "login", false, false, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved to be false")
	}
}

func TestRemoveWorktreeBranchDeleteFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			if strings.Contains(cmd, "worktree remove") {
				return "", nil
			}
			if strings.Contains(cmd, "branch -D") {
				return "", errors.New("branch in use")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login"}
	result, err := RemoveWorktree(r, wt, "login", false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v (partial success should not return error)", err)
	}
	if !result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved to be true")
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false")
	}
	if result.BranchError == nil {
		t.Fatal("expected BranchError to be set")
	}
	if !strings.Contains(result.BranchError.Error(), "git branch -D") {
		t.Errorf("expected recovery hint in error, got: %v", result.BranchError)
	}
}

func TestRemoveWorktreeProgressCallbacks(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	var messages []string
	onProgress := progress.Func(func(msg string) { messages = append(messages, msg) })

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login"}
	_, err := RemoveWorktree(r, wt, "login", false, false, onProgress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 progress messages, got %d: %v", len(messages), messages)
	}
	if messages[0] != "Removing worktree..." {
		t.Errorf("unexpected first message: %q", messages[0])
	}
	if messages[1] != "Deleting branch..." {
		t.Errorf("unexpected second message: %q", messages[1])
	}
}

func TestRemoveAndCleanupSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, err := removeAndCleanup(r, "/wt/test", "feature/test", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wtRemoved || !brDeleted {
		t.Errorf("expected (true, true), got (%v, %v)", wtRemoved, brDeleted)
	}
}

func TestRemoveAndCleanupWtRemovalFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("removal failed")
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, err := removeAndCleanup(r, "/wt/test", "feature/test", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if wtRemoved || brDeleted {
		t.Errorf("expected (false, false), got (%v, %v)", wtRemoved, brDeleted)
	}
}

func TestRemoveAndCleanupBranchDeleteFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			if strings.Contains(cmd, "branch -D") {
				return "", errors.New("branch in use")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, err := removeAndCleanup(r, "/wt/test", "feature/test", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !wtRemoved {
		t.Error("expected wtRemoved to be true")
	}
	if brDeleted {
		t.Error("expected brDeleted to be false")
	}
	if !strings.Contains(err.Error(), "failed to delete branch") {
		t.Errorf("expected unified hint in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "git branch -D feature/test") {
		t.Errorf("expected 'git branch -D feature/test' hint in error, got: %v", err)
	}
}

func TestRemoveAndCleanupForceFlag(t *testing.T) {
	tests := []struct {
		name      string
		force     bool
		wantForce bool
	}{
		{name: "force=true passes --force to git", force: true, wantForce: true},
		{name: "force=false omits --force from git", force: false, wantForce: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			r := &mockRunner{
				run: func(args ...string) (string, error) {
					if len(args) > 0 && args[0] == "worktree" {
						capturedArgs = args
					}
					return "", nil
				},
				runInDir: noopRunInDir,
			}
			_, _, err := removeAndCleanup(r, "/wt/test", "feature/test", tt.force)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			hasForce := slices.Contains(capturedArgs, "--force")
			if hasForce != tt.wantForce {
				t.Errorf("git worktree args %v: --force present=%v, want %v", capturedArgs, hasForce, tt.wantForce)
			}
		})
	}
}
