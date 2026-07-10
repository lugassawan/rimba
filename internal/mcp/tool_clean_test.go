package mcp

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Constants for repeated string literals in clean tests.
const (
	gitFetch       = "fetch"
	gitBranch      = "branch"
	gitLog         = "log"
	gitRemote      = "remote"
	gitRemotePrune = "remote prune"
	gitRemove      = "remove"
	gitMerged      = "--merged"
	gitFirstParent = "--first-parent"

	modeStale  = "stale"
	modeMerged = "merged"
	modePrune  = "prune"

	branchFeatureA    = "feature/a"
	branchFeatureDone = "feature/done"

	mergedFeatureDoneOutput = "  feature/done\n"

	// mainlineHistory simulates a merge-commit merge: worktreePorcelain's
	// fixed "HEAD abc123" is off this chain, so the entry counts as removable.
	mainlineHistory = "mainlineSha1\nmainlineSha2"
)

// mockCmdKey builds a dispatch key from git arguments.
// Two-arg commands like "worktree list" use "arg0 arg1"; single-arg commands use "arg0".
// Note: unmatched keys silently return ("", nil) — callers are responsible for handling
// only the args they declare and letting the remainder fall through harmlessly.
func mockCmdKey(args []string) string {
	if len(args) >= 2 {
		// git branch --merged=<branch> is a stuck-form single arg; normalize
		// it back to the "branch --merged" key regardless of which branch.
		if args[0] == gitBranch && strings.HasPrefix(args[1], gitMerged+"=") {
			return gitBranch + " " + gitMerged
		}
		return args[0] + " " + args[1]
	}
	if len(args) == 1 {
		return args[0]
	}
	return ""
}

// isBranchDelete returns true for "branch -D" or "branch -d" commands.
func isBranchDelete(args []string) bool {
	return len(args) >= 2 && args[0] == gitBranch && (args[1] == "-D" || args[1] == "-d")
}

// newCleanMergedRunner creates a mock runner for cleanMerged tests.
// porcelain is the worktree list output, mergedBranches is the branch --merged output,
// fetchOK controls whether fetch succeeds, and allowRemove controls whether
// worktree remove and branch delete are supported.
func newCleanMergedRunner(porcelain, mergedBranches string, fetchOK, allowRemove bool) *mockRunner {
	responses := map[string]string{
		gitBranch + " " + gitMerged:       mergedBranches,
		gitWorktree + " " + gitList:       porcelain,
		gitWorktree + " " + gitRemove:     "",
		gitRevList + " " + gitFirstParent: mainlineHistory,
	}

	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitFetch {
				if !fetchOK {
					return "", errors.New("fatal: could not fetch origin")
				}
				return "", nil
			}
			key := mockCmdKey(args)
			if out, ok := responses[key]; ok {
				if !allowRemove && (key == gitWorktree+" "+gitRemove) {
					return "", nil
				}
				return out, nil
			}
			if allowRemove && isBranchDelete(args) {
				return "", nil
			}
			return "", nil
		},
	}
}

// newCleanStaleRunner creates a mock runner for cleanStale tests.
// porcelain is the worktree list output, commitTimes maps branch names to
// timestamps, and allowRemove controls whether worktree remove and branch
// delete are supported.
func newCleanStaleRunner(porcelain string, commitTimes map[string]string, allowRemove bool) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			key := mockCmdKey(args)
			switch key {
			case gitWorktree + " " + gitList:
				return porcelain, nil
			case gitWorktree + " " + gitRemove:
				if allowRemove {
					return "", nil
				}
				return "", nil
			}
			if len(args) > 0 && args[0] == gitLog {
				return staleLogLookup(args, commitTimes), nil
			}
			if allowRemove && isBranchDelete(args) {
				return "", nil
			}
			return "", nil
		},
	}
}

// staleLogLookup returns the commit time output for a branch from the commitTimes map.
func staleLogLookup(args []string, commitTimes map[string]string) string {
	branch := args[len(args)-1]
	if ts, ok := commitTimes[branch]; ok {
		return ts
	}
	for _, ts := range commitTimes {
		return ts
	}
	return ""
}

func TestCleanToolRequiresMode(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleClean(hctx)

	result := callTool(t, handler, nil)
	errText := resultError(t, result)
	if !strings.Contains(errText, "mode is required") {
		t.Errorf("expected 'mode is required', got: %s", errText)
	}
}

func TestCleanToolInvalidMode(t *testing.T) {
	hctx := testContext(&mockRunner{})
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": "invalid"})
	errText := resultError(t, result)
	if !strings.Contains(errText, "invalid mode") {
		t.Errorf("expected 'invalid mode' error, got: %s", errText)
	}
}

func TestCleanToolPrune(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modePrune {
		t.Errorf("mode = %q, want %q", data.Mode, modePrune)
	}
}

func TestCleanToolPruneDryRun(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune, "dry_run": true})
	data := unmarshalJSON[cleanResult](t, result)
	if !data.DryRun {
		t.Error("expected dry_run=true")
	}
}

func TestCleanToolMergedNoWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := newCleanMergedRunner(porcelain, "", true, false)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeMerged})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modeMerged {
		t.Errorf("mode = %q, want %q", data.Mode, modeMerged)
	}
	if len(data.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(data.Removed))
	}
}

func TestCleanToolStaleNoWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return porcelain, nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modeStale {
		t.Errorf("mode = %q, want %q", data.Mode, modeStale)
	}
	if len(data.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(data.Removed))
	}
}

// --- New tests ---

func TestCleanToolPruneError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("fatal: unable to prune")
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	errText := resultError(t, result)
	if !strings.Contains(errText, modePrune) {
		t.Errorf("expected prune error, got: %s", errText)
	}
}

func TestCleanToolPruneWithOutput(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "Removing worktrees/stale-branch: gitdir file points to non-existent location", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modePrune {
		t.Errorf("mode = %q, want %q", data.Mode, modePrune)
	}
	if !strings.Contains(data.Output, "Removing") {
		t.Errorf("output = %q, expected prune output", data.Output)
	}
}

func TestCleanToolMergedWithCandidates(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-done", branchFeatureDone},
	)

	r := newCleanMergedRunner(porcelain, mergedFeatureDoneOutput, true, true)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeMerged})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modeMerged {
		t.Errorf("mode = %q, want %q", data.Mode, modeMerged)
	}
	if !data.DryRun && len(data.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(data.Removed))
	}
	if len(data.Removed) > 0 && data.Removed[0].Branch != branchFeatureDone {
		t.Errorf("removed branch = %q, want %q", data.Removed[0].Branch, branchFeatureDone)
	}
}

func TestCleanToolMergedDryRunWithCandidates(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-done", branchFeatureDone},
	)

	r := newCleanMergedRunner(porcelain, mergedFeatureDoneOutput, true, false)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeMerged, "dry_run": true})
	data := unmarshalJSON[cleanResult](t, result)
	if !data.DryRun {
		t.Error("expected dry_run=true")
	}
	if len(data.Removed) != 1 {
		t.Errorf("expected 1 candidate in dry run, got %d", len(data.Removed))
	}
	if len(data.Removed) > 0 && data.Removed[0].Branch != branchFeatureDone {
		t.Errorf("candidate branch = %q, want %q", data.Removed[0].Branch, branchFeatureDone)
	}
}

func TestCleanToolMergedFetchFails(t *testing.T) {
	// When fetch fails, cleanMerged should fall back to using mainBranch as mergeRef
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-done", branchFeatureDone},
	)

	r := newCleanMergedRunner(porcelain, mergedFeatureDoneOutput, false, true)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeMerged})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modeMerged {
		t.Errorf("mode = %q, want %q", data.Mode, modeMerged)
	}
	// Should still succeed despite fetch failure
	if len(data.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(data.Removed))
	}
}

func TestCleanToolMergedBranchListError(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitFetch {
				return "", nil
			}
			if len(args) >= 2 && args[0] == gitBranch && strings.HasPrefix(args[1], gitMerged+"=") {
				return "", errors.New("fatal: malformed object name")
			}
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeMerged})
	errText := resultError(t, result)
	if !strings.Contains(errText, "merged branches") {
		t.Errorf("expected merged branches error, got: %s", errText)
	}
}

func TestCleanToolStaleWithCandidates(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-old", "feature/old"},
	)

	// Timestamp from 30 days ago
	oldTime := time.Now().Add(-30 * 24 * time.Hour)
	oldTimestamp := strconv.FormatInt(oldTime.Unix(), 10)

	commitTimes := map[string]string{
		"feature/old": oldTimestamp + "\told commit message",
	}
	r := newCleanStaleRunner(porcelain, commitTimes, true)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale, "stale_days": 14})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modeStale {
		t.Errorf("mode = %q, want %q", data.Mode, modeStale)
	}
	if len(data.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(data.Removed))
	}
	if len(data.Removed) > 0 && data.Removed[0].Branch != "feature/old" {
		t.Errorf("removed branch = %q, want %q", data.Removed[0].Branch, "feature/old")
	}
}

func TestCleanToolStaleDryRunWithCandidates(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-old", "feature/old"},
	)

	oldTime := time.Now().Add(-30 * 24 * time.Hour)
	oldTimestamp := strconv.FormatInt(oldTime.Unix(), 10)

	commitTimes := map[string]string{
		"feature/old": oldTimestamp + "\told commit message",
	}
	r := newCleanStaleRunner(porcelain, commitTimes, false)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale, "stale_days": 14, "dry_run": true})
	data := unmarshalJSON[cleanResult](t, result)
	if !data.DryRun {
		t.Error("expected dry_run=true")
	}
	if len(data.Removed) != 1 {
		t.Errorf("expected 1 candidate in dry run, got %d", len(data.Removed))
	}
}

func TestCleanToolStaleRecentBranch(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-recent", "feature/recent"},
	)

	// Timestamp from 1 day ago (not stale)
	recentTime := time.Now().Add(-1 * 24 * time.Hour)
	recentTimestamp := strconv.FormatInt(recentTime.Unix(), 10)

	commitTimes := map[string]string{
		"feature/recent": recentTimestamp + "\trecent commit",
	}
	r := newCleanStaleRunner(porcelain, commitTimes, false)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale, "stale_days": 14})
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.Removed) != 0 {
		t.Errorf("expected 0 removed for recent branch, got %d", len(data.Removed))
	}
}

func TestCleanToolStaleListWorktreesError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return "", errors.New("fatal: not a git repository")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not a git repository") {
		t.Errorf("expected git error, got: %s", errText)
	}
}

func TestCleanToolMergedRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeMerged})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestCleanToolStaleRequiresConfig(t *testing.T) {
	hctx := &HandlerContext{
		Runner:   &mockRunner{},
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale})
	errText := resultError(t, result)
	if !strings.Contains(errText, "not initialized") {
		t.Errorf("expected config error, got: %s", errText)
	}
}

func TestCleanToolPruneNoConfig(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": "prune"})
	if result.IsError {
		t.Fatalf("prune should work without config, got error: %s", resultError(t, result))
	}
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != "prune" {
		t.Errorf("mode = %q, want %q", data.Mode, "prune")
	}
}

func TestCleanToolStaleLastCommitTimeError(t *testing.T) {
	// When LastCommitTime returns an error, the branch should be skipped
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-err", "feature/err"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == gitWorktree && args[1] == gitList {
				return porcelain, nil
			}
			if len(args) > 0 && args[0] == gitLog {
				return "", errors.New("fatal: bad object")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale, "stale_days": 14})
	data := unmarshalJSON[cleanResult](t, result)
	// Branch with error should be skipped, no candidates
	if len(data.Removed) != 0 {
		t.Errorf("expected 0 removed (error skipped), got %d", len(data.Removed))
	}
}

func TestCleanToolStaleMultipleBranches(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-old1", "feature/old1"},
		struct{ path, branch string }{"/wt/feature-recent", "feature/recent"},
		struct{ path, branch string }{"/wt/feature-old2", "feature/old2"},
	)

	oldTime := time.Now().Add(-30 * 24 * time.Hour)
	oldTimestamp := strconv.FormatInt(oldTime.Unix(), 10)
	recentTime := time.Now().Add(-1 * 24 * time.Hour)
	recentTimestamp := strconv.FormatInt(recentTime.Unix(), 10)

	commitTimes := map[string]string{
		"feature/old1":   oldTimestamp + "\told commit",
		"feature/recent": recentTimestamp + "\trecent commit",
		"feature/old2":   oldTimestamp + "\told commit",
	}
	r := newCleanStaleRunner(porcelain, commitTimes, true)
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale, "stale_days": 14})
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.Removed) != 2 {
		t.Fatalf("expected 2 removed (old1 and old2), got %d", len(data.Removed))
	}
	branches := make(map[string]bool)
	for _, r := range data.Removed {
		branches[r.Branch] = true
	}
	if !branches["feature/old1"] {
		t.Error("expected feature/old1 in removed")
	}
	if !branches["feature/old2"] {
		t.Error("expected feature/old2 in removed")
	}
	if branches["feature/recent"] {
		t.Error("feature/recent should not be in removed")
	}
}

func TestCleanToolMergedResolveMainBranchError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("no remote")
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   testConfig(),
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeMerged})
	if !result.IsError {
		t.Error("expected error when main branch can't be resolved")
	}
}

func TestCleanToolStaleResolveMainBranchError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("no remote")
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   testConfig(),
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modeStale})
	if !result.IsError {
		t.Error("expected error when main branch can't be resolved")
	}
}

func TestCleanToolPruneRemoteRefs(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			key := mockCmdKey(args)
			switch key {
			case gitRemote:
				return "origin\n", nil
			case gitRemotePrune:
				return " * [pruned] origin/x\n", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.RemotePruned) != 1 || data.RemotePruned[0] != "origin/x" {
		t.Errorf("RemotePruned = %v, want [origin/x]", data.RemotePruned)
	}
}

func TestCleanToolPruneNoOrigin(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) == 1 && args[0] == gitRemote {
				return "", nil // no remotes configured
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modePrune {
		t.Errorf("mode = %q, want %q", data.Mode, modePrune)
	}
	if len(data.RemotePruned) != 0 {
		t.Errorf("expected RemotePruned empty when no remotes, got %v", data.RemotePruned)
	}
}

func TestCleanToolPruneRemoteError(t *testing.T) {
	// When a remote prune fails, mcpCleanPrune records it as a warning (partial failure),
	// not as a tool error — the result should be successful with warnings populated.
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			key := mockCmdKey(args)
			switch key {
			case gitRemote:
				return "origin\n", nil
			case gitRemotePrune:
				return "", errors.New("connection refused")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	if result.IsError {
		t.Error("expected success result (warning) when remote prune fails; got error result")
	}
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.Warnings) == 0 {
		t.Error("expected Warnings to be populated on partial failure")
	}
}

func TestMcpCleanPruneListRemotesError(t *testing.T) {
	// When git.ListRemotes itself fails, mcpCleanPrune records a warning
	// and returns a success result with empty RemotePruned — not a tool error.
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if mockCmdKey(args) == gitRemote {
				return "", errors.New("not a git repository")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	if result.IsError {
		t.Error("expected success result when ListRemotes fails; got error result")
	}
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.RemotePruned) != 0 {
		t.Errorf("RemotePruned = %v, want empty when ListRemotes fails", data.RemotePruned)
	}
	if len(data.Warnings) == 0 {
		t.Error("expected Warnings to be populated when ListRemotes fails")
	}
	found := false
	for _, w := range data.Warnings {
		if strings.Contains(w, "list remotes") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Warnings = %v, want an entry mentioning 'list remotes'", data.Warnings)
	}
}

func TestMcpCleanPruneMultiRemote(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			key := mockCmdKey(args)
			switch key {
			case gitRemote:
				return "origin\nupstream\n", nil
			case gitRemotePrune:
				remote := args[len(args)-1]
				return " * [pruned] " + remote + "/gone\n", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	data := unmarshalJSON[cleanResult](t, result)
	if data.Mode != modePrune {
		t.Errorf("mode = %q, want %q", data.Mode, modePrune)
	}
	pruned := make(map[string]bool)
	for _, ref := range data.RemotePruned {
		pruned[ref] = true
	}
	if !pruned["origin/gone"] {
		t.Errorf("RemotePruned = %v, want origin/gone included", data.RemotePruned)
	}
	if !pruned["upstream/gone"] {
		t.Errorf("RemotePruned = %v, want upstream/gone included", data.RemotePruned)
	}
}

// newCleanMergedWithRemoteRunner builds a runner for merged-clean tests with remote support.
// If pushErr is nil, push --delete succeeds; otherwise it returns pushErr.
// pushCalled is incremented when push --delete is invoked (nil = ignore).
func newCleanMergedWithRemoteRunner(porcelain, mergedBranches string, pushErr error, pushCalled *int) *mockRunner {
	responses := map[string]string{
		gitBranch + " " + gitMerged:       mergedBranches,
		gitWorktree + " " + gitList:       porcelain,
		gitWorktree + " " + gitRemove:     "",
		gitRevList + " " + gitFirstParent: mainlineHistory,
	}

	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitFetch {
				return "", nil
			}
			if len(args) >= 3 && args[0] == gitRemote && args[1] == "get-url" {
				return "https://github.com/owner/repo.git", nil
			}
			if len(args) >= 3 && args[0] == "push" && args[2] == "--delete" {
				if pushCalled != nil {
					*pushCalled++
				}
				return "", pushErr
			}
			if out, ok := responses[mockCmdKey(args)]; ok {
				return out, nil
			}
			if isBranchDelete(args) {
				return "", nil
			}
			return "", nil
		},
	}
}

func TestCleanToolMergedDeletesRemoteBranch(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-done", branchFeatureDone},
	)
	pushCalled := 0
	r := newCleanMergedWithRemoteRunner(porcelain, mergedFeatureDoneOutput, nil, &pushCalled)

	result := callTool(t, handleClean(testContext(r)), map[string]any{"mode": modeMerged})
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(data.Removed))
	}
	if !data.Removed[0].RemoteDeleted {
		t.Error("expected remote_deleted=true in result")
	}
	if pushCalled == 0 {
		t.Error("expected git push --delete to be called")
	}
}

func TestCleanToolMergedRemoteFailureSurfacesInWarnings(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-done", branchFeatureDone},
	)
	r := newCleanMergedWithRemoteRunner(porcelain, mergedFeatureDoneOutput, errors.New("connection refused"), nil)

	result := callTool(t, handleClean(testContext(r)), map[string]any{"mode": modeMerged})
	if result.IsError {
		t.Fatal("expected success result even when remote delete fails")
	}
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.Warnings) == 0 {
		t.Error("expected Warnings to be populated when remote delete fails")
	}
	found := false
	for _, w := range data.Warnings {
		if strings.Contains(w, "remote") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Warnings = %v, expected an entry mentioning 'remote'", data.Warnings)
	}
	if len(data.Removed) != 1 || data.Removed[0].RemoteDeleted {
		t.Error("expected remote_deleted=false when push --delete fails")
	}
}

func TestMcpCleanPrunePartialFailure(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			key := mockCmdKey(args)
			switch key {
			case gitRemote:
				return "origin\nupstream\n", nil
			case gitRemotePrune:
				remote := args[len(args)-1]
				if remote == "upstream" {
					return "", errors.New("connection refused")
				}
				return " * [pruned] origin/gone\n", nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleClean(hctx)

	result := callTool(t, handler, map[string]any{"mode": modePrune})
	if result.IsError {
		t.Error("expected success result on partial failure; got error result")
	}
	data := unmarshalJSON[cleanResult](t, result)
	if len(data.RemotePruned) != 1 || data.RemotePruned[0] != "origin/gone" {
		t.Errorf("RemotePruned = %v, want [origin/gone]", data.RemotePruned)
	}
	if len(data.Warnings) == 0 {
		t.Error("expected Warnings to be populated for upstream failure")
	}
	found := false
	for _, w := range data.Warnings {
		if strings.Contains(w, "upstream") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Warnings = %v, want an entry mentioning 'upstream'", data.Warnings)
	}
}
