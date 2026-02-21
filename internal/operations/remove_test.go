package operations

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

func TestRemoveWorktree_Success(t *testing.T) {
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

func TestRemoveWorktree_KeepBranch(t *testing.T) {
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

func TestRemoveWorktree_RemovalFails(t *testing.T) {
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

func TestRemoveWorktree_BranchDeleteFails(t *testing.T) {
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

func TestRemoveWorktree_ProgressCallbacks(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	var messages []string
	progress := ProgressFunc(func(msg string) { messages = append(messages, msg) })

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login"}
	_, err := RemoveWorktree(r, wt, "login", false, false, progress)
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

func TestRemoveAndCleanup_Success(t *testing.T) {
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

func TestRemoveAndCleanup_WtRemovalFails(t *testing.T) {
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

func TestRemoveAndCleanup_BranchDeleteFails(t *testing.T) {
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
}
