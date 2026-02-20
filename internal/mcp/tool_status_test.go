package mcp

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStatusToolEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// DefaultBranch
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			// ListWorktrees: just main, no feature branches
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleStatus(hctx)

	result := callTool(t, handler, nil)
	data := unmarshalJSON[statusData](t, result)
	if data.Summary.Total != 0 {
		t.Errorf("expected 0 total, got %d", data.Summary.Total)
	}
	if len(data.Worktrees) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(data.Worktrees))
	}
	if data.StaleDays != 14 {
		t.Errorf("expected stale_days=14, got %d", data.StaleDays)
	}
}

func TestStatusToolWithWorktrees(t *testing.T) {
	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-login", "feature/login"},
	)

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return porcelain, nil
			}
			// LastCommitTime: return a date
			if len(args) > 0 && args[0] == gitLog {
				return "2025-01-01T00:00:00Z", nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			// status --porcelain: clean
			if len(args) > 0 && args[0] == gitStatus {
				return "", nil
			}
			// rev-list: no ahead/behind
			if len(args) > 0 && args[0] == gitRevList {
				return revListEven, nil
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleStatus(hctx)

	result := callTool(t, handler, map[string]any{"stale_days": 7})
	data := unmarshalJSON[statusData](t, result)
	if data.Summary.Total != 1 {
		t.Errorf("expected 1 total, got %d", data.Summary.Total)
	}
	if data.StaleDays != 7 {
		t.Errorf("expected stale_days=7, got %d", data.StaleDays)
	}
}

func TestStatusToolResolvesMainBranch(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			return "", nil
		},
	}

	// Config has DefaultSource — should use that
	hctx := testContext(r)
	hctx.Config.DefaultSource = "develop"
	handler := handleStatus(hctx)

	result := callTool(t, handler, nil)
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultError(t, result))
	}
}

func TestStatusToolNoConfig(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return worktreePorcelain(
					struct{ path, branch string }{"/repo", "main"},
				), nil
			}
			return "", nil
		},
	}
	hctx := &HandlerContext{
		Runner:   r,
		Config:   nil,
		RepoRoot: "/repo",
		Version:  "test",
	}
	handler := handleStatus(hctx)

	// Status should work without config (uses git detection)
	result := callTool(t, handler, nil)
	if result.IsError {
		errMsg := resultError(t, result)
		// Only fail if the error is about config — git detection errors are acceptable
		if strings.Contains(errMsg, "not initialized") {
			t.Errorf("status should work without config, got: %s", errMsg)
		}
	}
}

// newStaleDetectionRunner creates a mock runner for stale detection tests.
// porcelain is the worktree list output, and branchTimestamps maps branch
// names to their git log timestamp output.
func newStaleDetectionRunner(porcelain string, branchTimestamps map[string]string) *mockRunner {
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return porcelain, nil
			}
			if len(args) > 0 && args[0] == gitLog {
				for _, a := range args {
					if ts, ok := branchTimestamps[a]; ok {
						return ts, nil
					}
				}
				// Default: return the first available timestamp
				for _, ts := range branchTimestamps {
					return ts, nil
				}
				return "", nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitStatus {
				return "", nil
			}
			if len(args) > 0 && args[0] == gitRevList {
				return revListEven, nil
			}
			return "", nil
		},
	}
}

func TestStatusToolStaleDetection(t *testing.T) {
	// Unix timestamps for the log mock: LastCommitInfo expects "%ct\t%s" format.
	oldTS := strconv.FormatInt(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), 10)
	newTS := strconv.FormatInt(time.Now().Unix(), 10)

	porcelain := worktreePorcelain(
		struct{ path, branch string }{"/repo", "main"},
		struct{ path, branch string }{"/wt/feature-old", "feature/old"},
		struct{ path, branch string }{"/wt/feature-new", "feature/new"},
	)

	branchTimestamps := map[string]string{
		"feature/old": oldTS + "\told commit",
		"feature/new": newTS + "\tnew commit",
	}
	r := newStaleDetectionRunner(porcelain, branchTimestamps)
	hctx := testContext(r)
	handler := handleStatus(hctx)

	result := callTool(t, handler, map[string]any{"stale_days": 7})
	data := unmarshalJSON[statusData](t, result)
	if data.Summary.Total != 2 {
		t.Errorf("total = %d, want 2", data.Summary.Total)
	}
	if data.Summary.Stale != 1 {
		t.Errorf("stale = %d, want 1", data.Summary.Stale)
	}
	foundStale := false
	for _, w := range data.Worktrees {
		if w.Age != nil && w.Age.Stale {
			foundStale = true
		}
	}
	if !foundStale {
		t.Error("expected at least one stale worktree")
	}
}

// TestStatusToolSummaryCounters combines tests for dirty and behind summary counters
// using a table-driven approach.
func TestStatusToolSummaryCounters(t *testing.T) {
	tests := []struct {
		name       string
		dirtyOut   string // status --porcelain output
		revList    string // rev-list output
		wantDirty  int
		wantBehind int
	}{
		{
			name:       "dirty",
			dirtyOut:   " M dirty.go\n",
			revList:    revListEven,
			wantDirty:  1,
			wantBehind: 0,
		},
		{
			name:       "behind",
			dirtyOut:   "",
			revList:    "5\t0",
			wantDirty:  0,
			wantBehind: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recentTS := strconv.FormatInt(time.Now().Unix(), 10)

			porcelain := worktreePorcelain(
				struct{ path, branch string }{"/repo", "main"},
				struct{ path, branch string }{"/wt/feature-a", branchFeatureA},
			)

			r := &mockRunner{
				run: func(args ...string) (string, error) {
					if len(args) > 0 && args[0] == gitSymbolicRef {
						return refsOriginMain, nil
					}
					if len(args) > 0 && args[0] == gitWorktree {
						return porcelain, nil
					}
					if len(args) > 0 && args[0] == gitLog {
						return recentTS + "\tcommit msg", nil
					}
					return "", nil
				},
				runInDir: func(dir string, args ...string) (string, error) {
					if len(args) > 0 && args[0] == gitStatus {
						return tt.dirtyOut, nil
					}
					if len(args) > 0 && args[0] == gitRevList {
						return tt.revList, nil
					}
					return "", nil
				},
			}
			hctx := testContext(r)
			handler := handleStatus(hctx)

			result := callTool(t, handler, nil)
			data := unmarshalJSON[statusData](t, result)
			if data.Summary.Dirty != tt.wantDirty {
				t.Errorf("dirty = %d, want %d", data.Summary.Dirty, tt.wantDirty)
			}
			if data.Summary.Behind != tt.wantBehind {
				t.Errorf("behind = %d, want %d", data.Summary.Behind, tt.wantBehind)
			}
		})
	}
}

func TestStatusToolListError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == gitSymbolicRef {
				return refsOriginMain, nil
			}
			if len(args) > 0 && args[0] == gitWorktree {
				return "", errors.New("git error")
			}
			return "", nil
		},
	}
	hctx := testContext(r)
	handler := handleStatus(hctx)

	result := callTool(t, handler, nil)
	if !result.IsError {
		t.Error("expected error for list failure")
	}
}
