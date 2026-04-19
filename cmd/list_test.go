package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

func newListTestCmd() (*cobra.Command, *bytes.Buffer) {
	cmd, buf := newTestCmd()
	cmd.Flags().String(flagType, "", "")
	cmd.Flags().String(flagService, "", "")
	cmd.Flags().Bool(flagDirty, false, "")
	cmd.Flags().Bool(flagBehind, false, "")
	cmd.Flags().Bool(flagArchived, false, "")
	cmd.Flags().Bool(flagFull, false, "")
	return cmd, buf
}

func TestListInvalidType(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newListTestCmd()
	_ = cmd.Flags().Set(flagType, "invalid-type")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("error = %q, want 'invalid type'", err.Error())
	}
}

func TestListEmptyWorktrees(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	if !strings.Contains(buf.String(), "No worktrees found") {
		t.Errorf("output = %q, want 'No worktrees found'", buf.String())
	}
}

func TestListWithWorktrees(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "login") {
		t.Errorf("output = %q, want 'login' in table", out)
	}
}

func TestListFilterByType(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
		wtPrefix + repoDir + "/worktrees/bugfix-typo",
		"HEAD ghi789",
		"branch refs/heads/bugfix/typo",
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagType, "bugfix")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "typo") {
		t.Errorf("output = %q, want 'typo' in filtered table", out)
	}
}

func testListFilterNoMatch(t *testing.T, dirty, behind bool) {
	t.Helper()
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == cmdRevList {
				return aheadBehindZero, nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	if dirty {
		_ = cmd.Flags().Set(flagDirty, "true")
	}
	if behind {
		_ = cmd.Flags().Set(flagBehind, "true")
	}
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	if !strings.Contains(buf.String(), "No worktrees match the given filters") {
		t.Errorf("output = %q, want 'No worktrees match the given filters'", buf.String())
	}
}

func TestListDirtyFilter(t *testing.T) {
	testListFilterNoMatch(t, true, false)
}

func TestListBehindFilter(t *testing.T) {
	testListFilterNoMatch(t, false, true)
}

func TestValidPrefixType(t *testing.T) {
	if !resolver.ValidPrefixType("feature") {
		t.Error("expected 'feature' to be valid")
	}
	if resolver.ValidPrefixType("nonexistent") {
		t.Error("expected 'nonexistent' to be invalid")
	}
}

func TestListDirtyFilterWithMatch(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				// feature-login is dirty
				return dirtyOutput, nil
			}
			if len(args) >= 1 && args[0] == cmdRevList {
				return aheadBehindZero, nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagDirty, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if strings.Contains(out, "No worktrees match") {
		t.Errorf("output = %q, should NOT contain 'No worktrees match' when dirty worktrees exist", out)
	}
	if !strings.Contains(out, "login") {
		t.Errorf("output = %q, want 'login' in filtered table", out)
	}
}

func TestListArchivedBranches(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return branchListArchived, nil
			}
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					"worktree /wt/feature-active-task", "HEAD def456", "branch refs/heads/feature/active-task", "",
				}, "\n"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	cmd, buf := newTestCmd()

	err := listArchivedBranches(cmd, r, branchMain)
	if err != nil {
		t.Fatalf("listArchivedBranches: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Archived branches:") {
		t.Errorf("output = %q, want 'Archived branches:'", out)
	}
	if !strings.Contains(out, "archived-task") {
		t.Errorf("output = %q, want 'archived-task'", out)
	}
	if strings.Contains(out, "active-task") {
		t.Errorf("output = %q, should not contain 'active-task'", out)
	}
	if !strings.Contains(out, "rimba restore") {
		t.Errorf("output = %q, want restore hint", out)
	}
}

func TestListArchivedBranchesEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return branchMain, nil
			}
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return wtRepo + headMainBlock, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	cmd, buf := newTestCmd()

	err := listArchivedBranches(cmd, r, branchMain)
	if err != nil {
		t.Fatalf("listArchivedBranches: %v", err)
	}
	if !strings.Contains(buf.String(), "No archived branches found") {
		t.Errorf("output = %q, want 'No archived branches found'", buf.String())
	}
}

func TestListArchivedBranchesError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return "", errGitFailed
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	cmd, _ := newTestCmd()

	err := listArchivedBranches(cmd, r, branchMain)
	if err == nil {
		t.Fatal("expected error from ListArchivedBranches failure")
	}
}

func TestListJSONEmpty(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if env.Command != cmdList {
		t.Errorf("command = %q, want %q", env.Command, cmdList)
	}
	arr, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env.Data)
	}
	if len(arr) != 0 {
		t.Errorf("data length = %d, want 0", len(arr))
	}
}

func TestListJSONWithWorktrees(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Command != cmdList {
		t.Errorf("command = %q, want %q", env.Command, cmdList)
	}
	arr, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env.Data)
	}
	if len(arr) == 0 {
		t.Error("expected at least one worktree in JSON output")
	}
	// Verify first item has expected fields
	item, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("item type = %T, want map[string]any", arr[0])
	}
	if _, exists := item["task"]; !exists {
		t.Error("item missing 'task' field")
	}
	if _, exists := item["branch"]; !exists {
		t.Error("item missing 'branch' field")
	}
}

func TestListArchivedJSON(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if args[0] == cmdBranch {
				return branchListArchived, nil
			}
			if args[0] == cmdWorktreeTest && args[1] == cmdList {
				return strings.Join([]string{
					wtRepo, headABC123, branchRefMain, "",
					"worktree /wt/feature-active-task", "HEAD def456", "branch refs/heads/feature/active-task", "",
				}, "\n"), nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	cmd, buf := newTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")

	err := listArchivedBranches(cmd, r, branchMain)
	if err != nil {
		t.Fatalf("listArchivedBranches: %v", err)
	}

	var env output.Envelope
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if env.Command != cmdList {
		t.Errorf("command = %q, want %q", env.Command, cmdList)
	}
	arr, ok := env.Data.([]any)
	if !ok {
		t.Fatalf("data type = %T, want []any", env.Data)
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 archived branch, got %d", len(arr))
	}
}

func TestListCompactDefault(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "TASK") {
		t.Error("compact output should contain TASK header")
	}
	if !strings.Contains(out, "TYPE") {
		t.Error("compact output should contain TYPE header")
	}
	if !strings.Contains(out, "STATUS") {
		t.Error("compact output should contain STATUS header")
	}
	if strings.Contains(out, "BRANCH") {
		t.Error("compact output should NOT contain BRANCH header")
	}
	if strings.Contains(out, "PATH") {
		t.Error("compact output should NOT contain PATH header")
	}
}

func TestListFullMode(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagFull, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "TASK") {
		t.Error("full output should contain TASK header")
	}
	if !strings.Contains(out, "BRANCH") {
		t.Error("full output should contain BRANCH header")
	}
	if !strings.Contains(out, "PATH") {
		t.Error("full output should contain PATH header")
	}
	if !strings.Contains(out, "STATUS") {
		t.Error("full output should contain STATUS header")
	}
}

func TestListWithServiceColumn(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + "/worktrees/auth-api-feature-login",
		headDEF456,
		"branch refs/heads/auth-api/feature/login",
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "SERVICE") {
		t.Error("expected SERVICE column header when monorepo worktrees exist")
	}
	if !strings.Contains(out, "auth-api") {
		t.Error("expected auth-api in service column")
	}
}

func TestListWithServiceColumnFull(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + "/worktrees/auth-api-feature-login",
		headDEF456,
		"branch refs/heads/auth-api/feature/login",
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagFull, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if !strings.Contains(out, "SERVICE") {
		t.Error("expected SERVICE column in full mode")
	}
	if !strings.Contains(out, "BRANCH") {
		t.Error("expected BRANCH column in full mode")
	}
}

func TestListBehindFilterWithMatch(t *testing.T) {
	repoDir := t.TempDir()
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := strings.Join([]string{
		wtPrefix + repoDir,
		headABC123,
		branchRefMain,
		"",
		wtPrefix + repoDir + pathWorktreesFeatureLogin,
		headDEF456,
		branchRefFeatureLogin,
		"",
	}, "\n")

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			return worktreeOut, nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == cmdStatus {
				return "", nil
			}
			if len(args) >= 1 && args[0] == cmdRevList {
				// rev-list --left-right --count @{upstream}...HEAD
				// first field = upstream (behind), second field = HEAD (ahead)
				// 3 behind, 0 ahead
				return "3\t0", nil
			}
			return "", nil
		},
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagBehind, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	err := listCmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf(fatalListRunE, err)
	}
	out := buf.String()
	if strings.Contains(out, "No worktrees match") {
		t.Errorf("output = %q, should NOT contain 'No worktrees match' when behind worktrees exist", out)
	}
	if !strings.Contains(out, "login") {
		t.Errorf("output = %q, want 'login' in filtered table", out)
	}
}

func TestListValidateTypeEmpty(t *testing.T) {
	if err := listValidateType(""); err != nil {
		t.Errorf("expected nil for empty type, got %v", err)
	}
}

func TestListValidateTypeValid(t *testing.T) {
	if err := listValidateType("feature"); err != nil {
		t.Errorf("expected nil for valid type, got %v", err)
	}
}

func TestListReadFlags(t *testing.T) {
	cmd, _ := newListTestCmd()
	_ = cmd.Flags().Set(flagType, "feature")
	_ = cmd.Flags().Set(flagService, "web")
	_ = cmd.Flags().Set(flagDirty, "true")
	_ = cmd.Flags().Set(flagBehind, "true")
	_ = cmd.Flags().Set(flagArchived, "true")
	_ = cmd.Flags().Set(flagFull, "true")
	opts := listReadFlags(cmd)
	if opts.typeFilter != "feature" || opts.service != "web" || !opts.dirty || !opts.behind || !opts.archived || !opts.full {
		t.Errorf("unexpected opts: %+v", opts)
	}
}

func TestListRenderEmptyJSON(t *testing.T) {
	cmd, buf := newListTestCmd()
	_ = cmd.Flags().Set(flagJSON, "true")
	if err := listRenderEmpty(cmd, "none"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Errorf("output not JSON: %v", err)
	}
}

func TestListRenderEmptyText(t *testing.T) {
	cmd, buf := newListTestCmd()
	if err := listRenderEmpty(cmd, "none here"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(buf.String(), "none here") {
		t.Errorf("expected 'none here' in %q", buf.String())
	}
}

func TestListRenderTableWithFullAndService(t *testing.T) {
	cmd, buf := newListTestCmd()
	rows := []resolver.WorktreeDetail{
		{Task: "foo", Branch: "feature/foo", Type: "feature", Path: "/wt/foo", Service: "web", IsCurrent: true, Status: resolver.WorktreeStatus{}},
		{Task: "bar", Branch: "bugfix/bar", Type: "bugfix", Path: "/wt/bar", Status: resolver.WorktreeStatus{Dirty: true}},
	}
	listRenderTable(cmd, rows, true, nil, "")
	out := buf.String()
	for _, want := range []string{"foo", "bar", "feature/foo", "bugfix/bar", "SERVICE", "BRANCH", "PATH", "PR", "CI"} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in output: %s", want, out)
		}
	}
}

func TestFormatPRCell(t *testing.T) {
	p := termcolor.NewPainter(true)
	if got := formatPRCell(0, p); got != "–" {
		t.Errorf("0: got %q, want –", got)
	}
	if got := formatPRCell(412, p); got != "#412" {
		t.Errorf("412: got %q, want #412", got)
	}
}

func TestFormatCICell(t *testing.T) {
	p := termcolor.NewPainter(true)

	cases := map[string]struct {
		in   gh.CIStatus
		want string
	}{
		"empty":   {"", "–"},
		"success": {gh.CIStatusSuccess, "✓"},
		"pending": {gh.CIStatusPending, "●"},
		"failure": {gh.CIStatusFailure, "✗"},
		"unknown": {gh.CIStatus("WHATEVER"), "–"},
	}
	for name, tc := range cases {
		if got := formatCICell(tc.in, p); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestListRenderTableFullWithPRInfo(t *testing.T) {
	cmd, buf := newListTestCmd()
	rows := []resolver.WorktreeDetail{
		{Task: "a", Branch: "feature/a", Type: "feature", Path: "/wt/a", Status: resolver.WorktreeStatus{}},
	}
	info := map[string]operations.PRInfo{
		"feature/a": {Number: 777, CIStatus: gh.CIStatusSuccess},
	}

	listRenderTable(cmd, rows, true, info, "gh unavailable; PR/CI columns blank")
	out := buf.String()
	for _, want := range []string{"#777", "✓", "gh unavailable"} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in output: %s", want, out)
		}
	}
}

func TestListRenderJSONPopulatesPRInfo(t *testing.T) {
	cmd, buf := newListTestCmd()
	rows := []resolver.WorktreeDetail{
		{Task: "a", Branch: "feature/a", Type: "feature", Path: "/wt/a"},
	}
	info := map[string]operations.PRInfo{
		"feature/a": {Number: 9, CIStatus: gh.CIStatusPending},
	}
	if err := listRenderJSON(cmd, rows, info); err != nil {
		t.Fatalf("listRenderJSON: %v", err)
	}
	var payload struct {
		Data []output.ListItem `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("got %d items, want 1", len(payload.Data))
	}
	got := payload.Data[0]
	if got.PRNumber == nil || *got.PRNumber != 9 {
		t.Errorf("PRNumber = %v, want 9", got.PRNumber)
	}
	if got.CIStatus == nil || *got.CIStatus != "PENDING" {
		t.Errorf("CIStatus = %v, want PENDING", got.CIStatus)
	}
}
