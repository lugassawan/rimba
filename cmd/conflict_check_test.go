package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	branchFeatureA = "feature/a"
	branchFeatureB = "feature/b"
	branchFeatureC = "feature/c"
)

func testConflictCheckConfig() *config.Config {
	return &config.Config{DefaultSource: branchMain}
}

// worktreeListOutput returns mock output for `git worktree list --porcelain`.
func worktreeListOutput(branches ...string) string {
	var sb strings.Builder
	sb.WriteString("worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n")
	for i, b := range branches {
		sb.WriteString("worktree /wt/" + strings.ReplaceAll(b, "/", "-") + "\n")
		fmt.Fprintf(&sb, "HEAD def%03d56\n", i)
		sb.WriteString("branch refs/heads/" + b + "\n\n")
	}
	return sb.String()
}

func TestConflictCheckNoWorktrees(t *testing.T) {
	cmd, buf := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				// Only main worktree
				return "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := conflictCheckCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "No active worktree branches found") {
		t.Errorf("output = %q, want 'No active worktree branches found'", buf.String())
	}
}

func TestConflictCheckNoOverlaps(t *testing.T) {
	cmd, buf := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA, branchFeatureB), nil
			}
			if args[0] == cmdDiff {
				if strings.Contains(args[2], branchFeatureA) {
					return "file-a.go", nil
				}
				return "file-b.go", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := conflictCheckCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "No file overlaps found") {
		t.Errorf("output = %q, want 'No file overlaps found'", buf.String())
	}
}

func TestConflictCheckWithOverlaps(t *testing.T) {
	cmd, buf := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA, branchFeatureB), nil
			}
			if args[0] == cmdDiff {
				if strings.Contains(args[2], branchFeatureA) {
					return "shared.go\na-only.go", nil
				}
				return "shared.go\nb-only.go", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := conflictCheckCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "shared.go") {
		t.Errorf("output should contain 'shared.go', got:\n%s", out)
	}
	if !strings.Contains(out, "1 file overlap(s)") {
		t.Errorf("output should contain overlap count, got:\n%s", out)
	}
}

func TestConflictCheckHighSeverity(t *testing.T) {
	cmd, buf := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA, branchFeatureB, branchFeatureC), nil
			}
			if args[0] == cmdDiff {
				return "shared.go", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := conflictCheckCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "high (3)") {
		t.Errorf("output should contain 'high (3)' for 3-branch overlap, got:\n%s", out)
	}
}

func TestConflictCheckDryMerge(t *testing.T) {
	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagDryMerge, false, "")
	_ = cmd.Flags().Set(flagDryMerge, "true")
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	cmdMergeTree := "merge-tree"

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA, branchFeatureB), nil
			}
			if args[0] == cmdDiff {
				return "shared.go", nil
			}
			if args[0] == cmdMergeTree {
				conflictOut := "abc123\nCONFLICT (content): Merge conflict in shared.go"
				return conflictOut, errGitFailed
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := conflictCheckCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Dry merge conflicts") {
		t.Errorf("output should contain 'Dry merge conflicts', got:\n%s", out)
	}
	if !strings.Contains(out, "shared.go") {
		t.Errorf("output should contain conflict file 'shared.go', got:\n%s", out)
	}
}

func TestConflictCheckDiffError(t *testing.T) {
	cmd, _ := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA), nil
			}
			if args[0] == cmdDiff {
				return "", errGitFailed
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := conflictCheckCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error from diff failure")
	}
}
