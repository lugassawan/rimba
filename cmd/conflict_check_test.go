package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/termcolor"
)

const (
	branchFeatureA    = "feature/a"
	branchFeatureB    = "feature/b"
	branchFeatureC    = "feature/c"
	diffOutputFileA   = "file-a.go"
	diffOutputSharedA = "shared.go\na-only.go"
	diffOutputSharedB = "shared.go\nb-only.go"
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
					return diffOutputFileA, nil
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
					return diffOutputSharedA, nil
				}
				return diffOutputSharedB, nil
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

func TestHasConflictsTrue(t *testing.T) {
	results := []conflict.DryMergeResult{
		{Branch1: branchFeatureA, Branch2: branchFeatureB, HasConflicts: false},
		{Branch1: branchFeatureA, Branch2: branchFeatureC, HasConflicts: true, ConflictFiles: []string{"shared.go"}},
	}
	if !hasConflicts(results) {
		t.Error("hasConflicts should return true when at least one result has conflicts")
	}
}

func TestHasConflictsFalse(t *testing.T) {
	results := []conflict.DryMergeResult{
		{Branch1: branchFeatureA, Branch2: branchFeatureB, HasConflicts: false},
	}
	if hasConflicts(results) {
		t.Error("hasConflicts should return false when no results have conflicts")
	}
}

func TestHasConflictsEmpty(t *testing.T) {
	if hasConflicts(nil) {
		t.Error("hasConflicts should return false for nil input")
	}
}

func TestRenderDryMergeResultsNoConflicts(t *testing.T) {
	cmd, buf := newTestCmd()
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	results := []conflict.DryMergeResult{
		{Branch1: branchFeatureA, Branch2: branchFeatureB, HasConflicts: false},
	}

	renderDryMergeResults(cmd, p, results, nil)

	if buf.String() != "" {
		t.Errorf("expected empty output for no conflicts, got %q", buf.String())
	}
}

func TestRenderOverlapTableBranchWithoutPrefix(t *testing.T) {
	cmd, buf := newTestCmd()
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	result := &conflict.CheckResult{
		Overlaps: []conflict.FileOverlap{
			{
				File:     "shared.go",
				Branches: []string{"custom-branch", "another-branch"},
				Severity: conflict.SeverityLow,
			},
		},
		TotalBranches: 2,
	}

	renderOverlapTable(cmd, p, result, nil)
	out := buf.String()
	if !strings.Contains(out, "custom-branch") {
		t.Errorf("expected branch name without prefix, got: %s", out)
	}
}

func TestConflictCheckJSONNoOverlaps(t *testing.T) {
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA, branchFeatureB), nil
			}
			if args[0] == cmdDiff {
				if strings.Contains(args[2], branchFeatureA) {
					return diffOutputFileA, nil
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

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Command != "conflict-check" {
		t.Errorf("command = %q, want %q", env.Command, "conflict-check")
	}
	dataMap, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	overlaps, ok := dataMap["overlaps"].([]any)
	if !ok {
		t.Fatalf("overlaps type = %T, want []any", dataMap["overlaps"])
	}
	if len(overlaps) != 0 {
		t.Errorf("expected 0 overlaps, got %d", len(overlaps))
	}
}

func TestConflictCheckJSONWithOverlaps(t *testing.T) {
	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	cfg := testConflictCheckConfig()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return worktreeListOutput(branchFeatureA, branchFeatureB), nil
			}
			if args[0] == cmdDiff {
				if strings.Contains(args[2], branchFeatureA) {
					return diffOutputSharedA, nil
				}
				return diffOutputSharedB, nil
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

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	dataMap, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map[string]any", env.Data)
	}
	overlaps, ok := dataMap["overlaps"].([]any)
	if !ok {
		t.Fatalf("overlaps type = %T, want []any", dataMap["overlaps"])
	}
	if len(overlaps) != 1 {
		t.Errorf("expected 1 overlap, got %d", len(overlaps))
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
