package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	result, err := RemoveWorktree(context.Background(), r, wt, "login", false, false, nil)
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
	result, err := RemoveWorktree(context.Background(), r, wt, "login", true, false, nil)
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
	// .git present == genuine failure, not an orphan.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /somewhere/.git/worktrees/login\n"), 0o644); err != nil {
		t.Fatalf("failed to create .git fixture: %v", err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("worktree is dirty")
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: dir, Branch: "feature/login"}
	result, err := RemoveWorktree(context.Background(), r, wt, "login", false, false, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved to be false")
	}
	if result.LeftOnDisk {
		t.Error("expected result.LeftOnDisk to be false for a genuine (non-orphaned) removal failure")
	}
}

func TestRemoveWorktreePrunablePruneFails(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree remove"):
				return "", errors.New("remove failed")
			case strings.Contains(cmd, "worktree prune"):
				return "", errors.New("prune failed")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login", Prunable: true}
	result, err := RemoveWorktree(context.Background(), r, wt, "login", false, false, nil)
	if err == nil {
		t.Fatal("expected error when the post-repair remove and the prune fallback both fail")
	}
	if result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved to be false")
	}
	if result.BranchDeleted {
		t.Error("expected BranchDeleted to be false")
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
	result, err := RemoveWorktree(context.Background(), r, wt, "login", false, false, nil)
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
	_, err := RemoveWorktree(context.Background(), r, wt, "login", false, false, onProgress)
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

func TestRemoveWorktreePrunableInputHealsAndRemoves(t *testing.T) {
	var repairInvoked, removeInvoked, pruneInvoked bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree repair"):
				repairInvoked = true
			case strings.Contains(cmd, "worktree remove"):
				removeInvoked = true
			case strings.Contains(cmd, "worktree prune"):
				pruneInvoked = true
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login", Prunable: true}
	result, err := RemoveWorktree(context.Background(), r, wt, "login", false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WorktreeRemoved {
		t.Error("expected WorktreeRemoved to be true")
	}
	if !result.BranchDeleted {
		t.Error("expected BranchDeleted to be true")
	}
	if result.LeftOnDisk {
		t.Error("expected result.LeftOnDisk to be false — repair+remove fully cleared the directory")
	}
	if !repairInvoked {
		t.Error("expected 'git worktree repair' to be invoked for a prunable worktree")
	}
	if !removeInvoked {
		t.Error("expected 'git worktree remove' to be invoked after repair")
	}
	if pruneInvoked {
		t.Error("expected 'git worktree prune' NOT to be invoked when repair+remove succeed")
	}
}

func TestRemoveAndCleanupPrunableAllStepsFail(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree remove"):
				return "", errors.New("remove failed")
			case strings.Contains(cmd, "worktree prune"):
				return "", errors.New("prune failed")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, leftOnDisk, err := removeAndCleanup(context.Background(), r, "/wt/test", "feature/test", false, true)
	if err == nil {
		t.Fatal("expected error when the post-repair remove and the prune fallback both fail")
	}
	if wtRemoved || brDeleted {
		t.Errorf("expected (false, false), got (%v, %v)", wtRemoved, brDeleted)
	}
	if !leftOnDisk {
		t.Error("expected leftOnDisk to be true when all recovery steps fail")
	}
}

func TestRemoveAndCleanupSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, _, err := removeAndCleanup(context.Background(), r, "/wt/test", "feature/test", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wtRemoved || !brDeleted {
		t.Errorf("expected (true, true), got (%v, %v)", wtRemoved, brDeleted)
	}
}

func TestRemoveAndCleanupWtRemovalFails(t *testing.T) {
	// .git present == genuine failure, not an orphan.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /somewhere/.git/worktrees/test\n"), 0o644); err != nil {
		t.Fatalf("failed to create .git fixture: %v", err)
	}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("removal failed")
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, leftOnDisk, err := removeAndCleanup(context.Background(), r, dir, "feature/test", false, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if wtRemoved || brDeleted {
		t.Errorf("expected (false, false), got (%v, %v)", wtRemoved, brDeleted)
	}
	if leftOnDisk {
		t.Error("expected leftOnDisk to be false for a genuine (non-orphaned) removal failure")
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

	wtRemoved, brDeleted, _, err := removeAndCleanup(context.Background(), r, "/wt/test", "feature/test", false, false)
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

func TestRemoveAndCleanupPrunableInputHealsAndRemoves(t *testing.T) {
	var repairInvoked, removeInvoked, pruneInvoked bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree repair"):
				repairInvoked = true
			case strings.Contains(cmd, "worktree remove"):
				removeInvoked = true
			case strings.Contains(cmd, "worktree prune"):
				pruneInvoked = true
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, leftOnDisk, err := removeAndCleanup(context.Background(), r, "/wt/test", "feature/test", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wtRemoved || !brDeleted {
		t.Errorf("expected (true, true), got (%v, %v)", wtRemoved, brDeleted)
	}
	if leftOnDisk {
		t.Error("expected leftOnDisk to be false — repair+remove fully cleared the directory")
	}
	if !repairInvoked {
		t.Error("expected 'git worktree repair' to be invoked for a prunable worktree")
	}
	if !removeInvoked {
		t.Error("expected 'git worktree remove' to be invoked after repair")
	}
	if pruneInvoked {
		t.Error("expected 'git worktree prune' NOT to be invoked when repair+remove succeed")
	}
}

func TestRemoveAndCleanupOrphanedRepairsThenRemoves(t *testing.T) {
	dir := t.TempDir() // no .git — orphaned

	var repairInvoked bool
	removeCalls := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree repair"):
				repairInvoked = true
				return "", nil
			case strings.Contains(cmd, "worktree remove"):
				removeCalls++
				if removeCalls == 1 {
					return "", errors.New("validation failed, cannot remove working tree")
				}
				return "", nil
			}
			return "", nil // branch -D
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, leftOnDisk, err := removeAndCleanup(context.Background(), r, dir, "feature/test", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wtRemoved || !brDeleted {
		t.Errorf("expected (true, true), got (%v, %v)", wtRemoved, brDeleted)
	}
	if leftOnDisk {
		t.Error("expected leftOnDisk to be false after repair+remove succeed")
	}
	if !repairInvoked {
		t.Error("expected 'git worktree repair' to be invoked to heal the orphaned worktree")
	}
	if removeCalls != 2 {
		t.Errorf("expected 'git worktree remove' to be invoked twice (initial fail, post-repair retry), got %d", removeCalls)
	}
}

func TestRemoveAndCleanupOrphanedRepairFailsFallsBackToPrune(t *testing.T) {
	dir := t.TempDir() // no .git — orphaned

	var pruneInvoked bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree repair"):
				return "", errors.New("repair failed")
			case strings.Contains(cmd, "worktree remove"):
				return "", errors.New("validation failed, cannot remove working tree")
			case strings.Contains(cmd, "worktree prune"):
				pruneInvoked = true
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, leftOnDisk, err := removeAndCleanup(context.Background(), r, dir, "feature/test", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wtRemoved || !brDeleted {
		t.Errorf("expected (true, true) — prune fallback still clears the registration, got (%v, %v)", wtRemoved, brDeleted)
	}
	if !leftOnDisk {
		t.Error("expected leftOnDisk to be true after falling back to prune")
	}
	if !pruneInvoked {
		t.Error("expected 'git worktree prune' fallback to be invoked")
	}
}

func TestRemoveAndCleanupOrphanedForceFalseDirtySurfacesError(t *testing.T) {
	dir := t.TempDir() // no .git — orphaned

	var capturedRemoveArgs []string
	removeCalls := 0
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "worktree repair"):
				// Simulate repair actually fixing the .git file, as it does in production.
				if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /somewhere/.git/worktrees/test\n"), 0o644); err != nil {
					t.Fatalf("failed to simulate repair: %v", err)
				}
				return "", nil
			case strings.Contains(cmd, "worktree remove"):
				removeCalls++
				capturedRemoveArgs = args
				return "", errors.New("worktree contains modified or untracked files, use --force")
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wtRemoved, brDeleted, leftOnDisk, err := removeAndCleanup(context.Background(), r, dir, "feature/test", false, false)
	if err == nil {
		t.Fatal("expected the genuine dirty-worktree error to surface")
	}
	if wtRemoved || brDeleted {
		t.Errorf("expected (false, false), got (%v, %v)", wtRemoved, brDeleted)
	}
	if leftOnDisk {
		t.Error("expected leftOnDisk to be false — the worktree is still fully valid, not left behind by a prune")
	}
	if slices.Contains(capturedRemoveArgs, "--force") {
		t.Error("expected --force NOT to be passed to the post-repair remove when the caller's force is false")
	}
	if removeCalls != 2 {
		t.Errorf("expected 2 remove attempts (initial fail, post-repair retry), got %d", removeCalls)
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
			_, _, _, err := removeAndCleanup(context.Background(), r, "/wt/test", "feature/test", tt.force, false)
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

func TestRemoveWorktreeWritesAndCleansSweepManifest(t *testing.T) {
	commonDir := t.TempDir()
	var sawManifestDuringRemoval bool
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			cmd := strings.Join(args, " ")
			switch {
			case strings.Contains(cmd, "rev-parse --git-common-dir"):
				return commonDir, nil
			case strings.Contains(cmd, "worktree remove"):
				matches, _ := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
				sawManifestDuringRemoval = len(matches) == 1
				return "", nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	wt := resolver.WorktreeInfo{Path: "/wt/feature-login", Branch: "feature/login"}
	if _, err := RemoveWorktree(context.Background(), r, wt, "login", false, false, nil); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	if !sawManifestDuringRemoval {
		t.Error("expected a sweep manifest to exist while git worktree remove was running")
	}
	matches, _ := filepath.Glob(filepath.Join(commonDir, sweepManifestDir, "sweep-*.json"))
	if len(matches) != 0 {
		t.Errorf("expected manifest cleaned up after RemoveWorktree returns, got %v", matches)
	}
}

func TestWorktreeGitMissing(t *testing.T) {
	tests := []struct {
		name        string
		setupGit    bool
		wantMissing bool
	}{
		{name: ".git present", setupGit: true, wantMissing: false},
		{name: ".git absent", setupGit: false, wantMissing: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.setupGit {
				if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /x\n"), 0o644); err != nil {
					t.Fatalf("failed to create .git fixture: %v", err)
				}
			}
			if got := worktreeGitMissing(dir); got != tt.wantMissing {
				t.Errorf("worktreeGitMissing(%q) = %v, want %v", dir, got, tt.wantMissing)
			}
		})
	}
}
