package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
)

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

	// Save and restore module-level vars
	origType := listType
	defer func() { listType = origType }()
	listType = "invalid-type"

	cmd, _ := newTestCmd()
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

	origType := listType
	origDirty := listDirty
	origBehind := listBehind
	defer func() {
		listType = origType
		listDirty = origDirty
		listBehind = origBehind
	}()
	listType = ""
	listDirty = false
	listBehind = false

	cmd, buf := newTestCmd()
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

	origType := listType
	origDirty := listDirty
	origBehind := listBehind
	defer func() {
		listType = origType
		listDirty = origDirty
		listBehind = origBehind
	}()
	listType = ""
	listDirty = false
	listBehind = false

	cmd, buf := newTestCmd()
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

	origType := listType
	origDirty := listDirty
	origBehind := listBehind
	defer func() {
		listType = origType
		listDirty = origDirty
		listBehind = origBehind
	}()
	listType = "bugfix"
	listDirty = false
	listBehind = false

	cmd, buf := newTestCmd()
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

	origType := listType
	origDirty := listDirty
	origBehind := listBehind
	defer func() {
		listType = origType
		listDirty = origDirty
		listBehind = origBehind
	}()
	listType = ""
	listDirty = dirty
	listBehind = behind

	cmd, buf := newTestCmd()
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

	origType := listType
	origDirty := listDirty
	origBehind := listBehind
	defer func() {
		listType = origType
		listDirty = origDirty
		listBehind = origBehind
	}()
	listType = ""
	listDirty = true
	listBehind = false

	cmd, buf := newTestCmd()
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
				return "main", nil
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
		t.Fatal("expected error from findArchivedBranches failure")
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

	origType := listType
	origDirty := listDirty
	origBehind := listBehind
	defer func() {
		listType = origType
		listDirty = origDirty
		listBehind = origBehind
	}()
	listType = ""
	listDirty = false
	listBehind = true

	cmd, buf := newTestCmd()
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
