package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestMergePlanNoWorktrees(t *testing.T) {
	cmd, buf := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := mergePlanCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !strings.Contains(buf.String(), "No active worktree branches found") {
		t.Errorf("output = %q, want 'No active worktree branches found'", buf.String())
	}
}

func TestMergePlanSingleBranch(t *testing.T) {
	cmd, buf := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA), nil
			}
			if args[0] == cmdDiff {
				return "file-a.go", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := mergePlanCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Merge in this order") {
		t.Errorf("output should contain merge order instruction, got:\n%s", out)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("output should contain order number, got:\n%s", out)
	}
}

func TestMergePlanOrdering(t *testing.T) {
	cmd, buf := newTestCmd()
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA, branchFeatureB, branchFeatureC), nil
			}
			if args[0] == cmdDiff {
				// A and B share shared.go, C has no overlaps
				if strings.Contains(args[2], branchFeatureA) {
					return "shared.go\na-only.go", nil
				}
				if strings.Contains(args[2], branchFeatureB) {
					return "shared.go\nb-only.go", nil
				}
				return "c-only.go", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	err := mergePlanCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()

	// C should be first (0 conflicts with others)
	lines := strings.Split(out, "\n")
	// Find data rows: start with a digit (order number)
	var dataLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' {
			dataLines = append(dataLines, trimmed)
		}
	}

	if len(dataLines) < 3 {
		t.Fatalf("expected at least 3 data lines, got %d:\n%s", len(dataLines), out)
	}

	// First data line should start with "1" and contain "c" (feature/c has fewest conflicts)
	if !strings.HasPrefix(dataLines[0], "1") || !strings.Contains(dataLines[0], "c") {
		t.Errorf("first merge should be feature/c, got: %s", dataLines[0])
	}
}

func TestMergePlanDiffError(t *testing.T) {
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

	err := mergePlanCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error from diff failure")
	}
}
