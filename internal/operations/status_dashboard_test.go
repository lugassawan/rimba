package operations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// statusDashboardRunner builds a mockRunner backed by configurable worktree data.
type statusDashboardRunner struct {
	porcelain    string
	dirtyDirs    map[string]bool
	behindByDir  map[string]int    // dir → behind count
	commitTimes  map[string]string // branch → unix timestamp string; "" means error
	commitCounts map[string]string // branch → count string; "" means error
}

func (r *statusDashboardRunner) build() *mockRunner {
	return &mockRunner{run: r.run, runInDir: r.runInDir}
}

func (r *statusDashboardRunner) handleLog(args []string) (string, error) {
	branch := args[len(args)-1]
	ts, ok := r.commitTimes[branch]
	if !ok || ts == "" {
		return "", errGitFailed
	}
	return ts + "\t-", nil
}

func (r *statusDashboardRunner) handleRevListCount(args []string) (string, error) {
	branch := args[len(args)-1]
	c, ok := r.commitCounts[branch]
	if !ok {
		return "0", nil
	}
	if c == "" {
		return "", errGitFailed
	}
	return c, nil
}

func (r *statusDashboardRunner) run(args ...string) (string, error) {
	switch {
	case len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList:
		return r.porcelain, nil
	case len(args) >= 1 && args[0] == "log":
		return r.handleLog(args)
	case len(args) >= 2 && args[0] == gitCmdRevList && args[1] == "--count":
		return r.handleRevListCount(args)
	case len(args) >= 1 && args[0] == "symbolic-ref":
		return refsRemotesOriginMain, nil
	}
	return "", nil
}

func (r *statusDashboardRunner) runInDir(dir string, args ...string) (string, error) {
	if len(args) >= 1 && args[0] == gitCmdStatus {
		if r.dirtyDirs[dir] {
			return "M file.go", nil
		}
		return "", nil
	}
	if len(args) >= 1 && args[0] == gitCmdRevList {
		if behind, ok := r.behindByDir[dir]; ok && behind > 0 {
			return string(rune('0'+behind)) + "\t0", nil
		}
		return "0\t0", nil
	}
	return "", nil
}

func TestStatusDashboardEmpty(t *testing.T) {
	porcelain := porcelainEntries(struct{ path, branch string }{pathMainRepo, "main"})
	r := (&statusDashboardRunner{
		porcelain:   porcelain,
		commitTimes: map[string]string{},
	}).build()

	res, err := StatusDashboard(context.Background(), r, StatusDashboardRequest{})
	if err != nil {
		t.Fatalf("StatusDashboard: %v", err)
	}
	if len(res.Entries) != 0 {
		t.Errorf("Entries = %d, want 0", len(res.Entries))
	}
	if res.Footprint != nil {
		t.Error("Footprint should be nil when no candidates")
	}
}

func TestStatusDashboardCollectsStatuses(t *testing.T) {
	porcelain := porcelainEntries(
		struct{ path, branch string }{pathMainRepo, "main"},
		struct{ path, branch string }{pathWtFeatureAuth, branchFeatureAuth},
		struct{ path, branch string }{pathWtBugfixLogin, branchBugfixLogin},
		struct{ path, branch string }{pathWtChoreLogout, branchChoreLogout},
	)
	fixedTS := "1700000000"
	r := (&statusDashboardRunner{
		porcelain:   porcelain,
		dirtyDirs:   map[string]bool{pathWtFeatureAuth: true},
		behindByDir: map[string]int{pathWtFeatureAuth: 2},
		commitTimes: map[string]string{
			branchFeatureAuth: fixedTS,
			branchBugfixLogin: fixedTS,
			branchChoreLogout: fixedTS,
		},
	}).build()

	res, err := StatusDashboard(context.Background(), r, StatusDashboardRequest{})
	if err != nil {
		t.Fatalf("StatusDashboard: %v", err)
	}
	if len(res.Entries) != 3 {
		t.Fatalf("Entries = %d, want 3", len(res.Entries))
	}
	if res.Footprint != nil {
		t.Error("Footprint should be nil when Detail=false")
	}

	var featureEntry *StatusEntry
	for i := range res.Entries {
		if res.Entries[i].Entry.Branch == branchFeatureAuth {
			featureEntry = &res.Entries[i]
			break
		}
	}
	if featureEntry == nil {
		t.Fatal("feature/auth entry not found in results")
	}
	if !featureEntry.Status.Dirty {
		t.Error("feature/auth: expected Dirty=true")
	}
	if featureEntry.Status.Behind != 2 {
		t.Errorf("feature/auth: Behind = %d, want 2", featureEntry.Status.Behind)
	}
	if !featureEntry.HasTime {
		t.Error("feature/auth: expected HasTime=true")
	}
}

func TestStatusDashboardDetailPopulatesSizesAndVelocity(t *testing.T) {
	mainDir := t.TempDir()
	bigDir := t.TempDir()
	smallDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(bigDir, "blob"), make([]byte, 8*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(smallDir, "tiny"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	branchBig := "feature/big"
	branchSmall := "feature/small"
	porcelain := porcelainEntries(
		struct{ path, branch string }{mainDir, "main"},
		struct{ path, branch string }{bigDir, branchBig},
		struct{ path, branch string }{smallDir, branchSmall},
	)
	r := (&statusDashboardRunner{
		porcelain: porcelain,
		commitTimes: map[string]string{
			branchBig:   "1700000000",
			branchSmall: "1700000000",
		},
		commitCounts: map[string]string{
			branchBig:   "9",
			branchSmall: "3",
		},
	}).build()

	res, err := StatusDashboard(context.Background(), r, StatusDashboardRequest{Detail: true})
	if err != nil {
		t.Fatalf("StatusDashboard detail: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("Entries = %d, want 2", len(res.Entries))
	}
	if res.Footprint == nil {
		t.Fatal("Footprint should be non-nil when Detail=true")
	}
	if res.Entries[0].Entry.Branch != branchBig {
		t.Errorf("Entries[0].Branch = %q, want %q (sort by size desc)", res.Entries[0].Entry.Branch, branchBig)
	}
	for _, e := range res.Entries {
		if e.SizeBytes == nil {
			t.Errorf("branch %q: SizeBytes is nil, expected populated", e.Entry.Branch)
		}
		if e.Recent7D == nil {
			t.Errorf("branch %q: Recent7D is nil, expected populated", e.Entry.Branch)
		}
	}
	if res.Footprint.MainErr != nil {
		t.Errorf("Footprint.MainErr = %v, want nil", res.Footprint.MainErr)
	}
	if res.Footprint.TotalBytes == 0 {
		t.Error("Footprint.TotalBytes should be non-zero")
	}
}

func TestStatusDashboardDetailMainSizeErrorPropagates(t *testing.T) {
	nonexistent := "/nonexistent/path/that/does/not/exist/xyz123"
	bigDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(bigDir, "blob"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}

	branchFeat := "feature/x"
	porcelain := porcelainEntries(
		struct{ path, branch string }{nonexistent, "main"},
		struct{ path, branch string }{bigDir, branchFeat},
	)
	r := (&statusDashboardRunner{
		porcelain:   porcelain,
		commitTimes: map[string]string{branchFeat: "1700000000"},
	}).build()

	res, err := StatusDashboard(context.Background(), r, StatusDashboardRequest{Detail: true})
	if err != nil {
		t.Fatalf("StatusDashboard: %v", err)
	}
	if res.Footprint == nil {
		t.Fatal("Footprint should be non-nil with Detail=true")
	}
	if res.Footprint.MainErr == nil {
		t.Error("Footprint.MainErr should be non-nil when main path is invalid")
	}
	if res.Footprint.TotalBytes == 0 {
		t.Error("Footprint.TotalBytes should include worktree bytes even when main errors")
	}
}

func TestStatusDashboardLastCommitTimeErrorIsNonFatal(t *testing.T) {
	porcelain := porcelainEntries(
		struct{ path, branch string }{pathMainRepo, "main"},
		struct{ path, branch string }{pathWtFeatureAuth, branchFeatureAuth},
		struct{ path, branch string }{pathWtBugfixLogin, branchBugfixLogin},
	)
	r := (&statusDashboardRunner{
		porcelain: porcelain,
		commitTimes: map[string]string{
			branchFeatureAuth: "1700000000",
			branchBugfixLogin: "",
		},
	}).build()

	res, err := StatusDashboard(context.Background(), r, StatusDashboardRequest{})
	if err != nil {
		t.Fatalf("StatusDashboard: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("Entries = %d, want 2", len(res.Entries))
	}

	for _, e := range res.Entries {
		if e.Entry.Branch == branchFeatureAuth && !e.HasTime {
			t.Errorf("feature/auth: expected HasTime=true")
		}
		if e.Entry.Branch == branchBugfixLogin && e.HasTime {
			t.Errorf("bugfix/login: expected HasTime=false on commit-time error")
		}
	}
}

func TestStatusDashboardGitListError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "symbolic-ref" {
				return refsRemotesOriginMain, nil
			}
			return "", errGitFailed
		},
		runInDir: noopRunInDir,
	}
	_, err := StatusDashboard(context.Background(), r, StatusDashboardRequest{})
	if err == nil {
		t.Fatal("expected error when git.ListWorktrees fails")
	}
}

func TestStatusDashboardDetailNilMainEntry(t *testing.T) {
	// Candidates exist but no main-branch worktree → mainEntry == nil,
	// so the main-size goroutine must not be launched.
	branchFeat := "feature/y"
	porcelain := porcelainEntries(
		struct{ path, branch string }{pathWtFeatureAuth, branchFeat},
	)
	r := (&statusDashboardRunner{
		porcelain:   porcelain,
		commitTimes: map[string]string{branchFeat: "1700000000"},
	}).build()

	res, err := StatusDashboard(context.Background(), r, StatusDashboardRequest{Detail: true})
	if err != nil {
		t.Fatalf("StatusDashboard: %v", err)
	}
	if len(res.Entries) != 1 {
		t.Fatalf("Entries = %d, want 1", len(res.Entries))
	}
	if res.Footprint == nil {
		t.Fatal("Footprint should be non-nil with Detail=true")
	}
}

func TestSortEntriesBySizeDesc(t *testing.T) {
	ptr := func(v int64) *int64 { return &v }

	entries := []StatusEntry{
		{Entry: git.WorktreeEntry{Branch: "small"}, SizeBytes: ptr(100)},
		{Entry: git.WorktreeEntry{Branch: "errored"}, SizeBytes: nil},
		{Entry: git.WorktreeEntry{Branch: "big"}, SizeBytes: ptr(5000)},
		{Entry: git.WorktreeEntry{Branch: "medium"}, SizeBytes: ptr(1000)},
		{Entry: git.WorktreeEntry{Branch: "errored2"}, SizeBytes: nil},
	}
	sortEntriesBySizeDesc(entries)
	wantOrder := []string{"big", "medium", "small", "errored", "errored2"}
	for i, w := range wantOrder {
		if entries[i].Entry.Branch != w {
			t.Errorf("index %d: got %q, want %q", i, entries[i].Entry.Branch, w)
		}
	}
}

func TestSortEntriesBySizeDescAllNil(t *testing.T) {
	entries := []StatusEntry{
		{Entry: git.WorktreeEntry{Branch: "a"}},
		{Entry: git.WorktreeEntry{Branch: "b"}},
	}
	sortEntriesBySizeDesc(entries)
	if entries[0].Entry.Branch != "a" || entries[1].Entry.Branch != "b" {
		t.Errorf("expected stable order [a, b], got [%s, %s]", entries[0].Entry.Branch, entries[1].Entry.Branch)
	}
}

// resolverStatus is a small helper to build a WorktreeStatus for test entries.
func resolverStatus(dirty bool, behind int) resolver.WorktreeStatus {
	return resolver.WorktreeStatus{Dirty: dirty, Behind: behind}
}

func TestSummarizeStatus(t *testing.T) {
	now := time.Now()
	staleThreshold := now.Add(-14 * 24 * time.Hour)
	oldCommit := now.Add(-30 * 24 * time.Hour)
	recentCommit := now.Add(-1 * time.Hour)

	entry := func(dirty bool, behind int, ct time.Time, hasTime bool) StatusEntry {
		return StatusEntry{
			Entry:      git.WorktreeEntry{Branch: "feature/x"},
			Status:     resolverStatus(dirty, behind),
			CommitTime: ct,
			HasTime:    hasTime,
		}
	}

	cases := []struct {
		name    string
		entries []StatusEntry
		want    StatusSummary
	}{
		{name: "empty", entries: nil, want: StatusSummary{}},
		{
			name:    "all zero",
			entries: []StatusEntry{entry(false, 0, recentCommit, true), entry(false, 0, recentCommit, true)},
			want:    StatusSummary{Total: 2},
		},
		{
			name:    "dirty only",
			entries: []StatusEntry{entry(true, 0, recentCommit, true), entry(false, 0, recentCommit, true)},
			want:    StatusSummary{Total: 2, Dirty: 1},
		},
		{
			name:    "behind only",
			entries: []StatusEntry{entry(false, 3, recentCommit, true), entry(false, 0, recentCommit, true)},
			want:    StatusSummary{Total: 2, Behind: 1},
		},
		{
			name:    "stale only",
			entries: []StatusEntry{entry(false, 0, oldCommit, true), entry(false, 0, recentCommit, true)},
			want:    StatusSummary{Total: 2, Stale: 1},
		},
		{
			name:    "hasTime=false must not count as stale",
			entries: []StatusEntry{entry(false, 0, oldCommit, false)},
			want:    StatusSummary{Total: 1},
		},
		{
			name: "mix of all",
			entries: []StatusEntry{
				entry(true, 2, oldCommit, true),
				entry(false, 0, recentCommit, true),
				entry(true, 0, oldCommit, false),
			},
			want: StatusSummary{Total: 3, Dirty: 2, Behind: 1, Stale: 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SummarizeStatus(tc.entries, staleThreshold)
			if got != tc.want {
				t.Errorf("SummarizeStatus = %+v, want %+v", got, tc.want)
			}
		})
	}
}
