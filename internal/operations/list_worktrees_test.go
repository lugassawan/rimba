package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/gh"
)

const (
	branchBugfixLogin  = "bugfix/login"
	branchChoreLogout  = "chore/logout"
	branchFeatureAuth  = "feature/auth"
	pathWtFeatureAuth  = "/wt/feature-auth"
	pathWtBugfixLogin  = "/wt/bugfix-login"
	pathWtChoreLogout  = "/wt/chore-logout"
	wtDirTest          = "/wt"
	dirtyStatusFixture = "M file.go"
	prListEntrySuccess = `[{"number":12,"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}]`
	prListEntryPending = `[{"number":13,"statusCheckRollup":[{"status":"IN_PROGRESS","conclusion":""}]}]`
	prListEntryFailure = `[{"number":14,"statusCheckRollup":[{"status":"COMPLETED","conclusion":"FAILURE"}]}]`
	prListEntryNoCheck = `[{"number":15,"statusCheckRollup":[]}]`
	prListNoOpenPR     = `[]`
)

// threeWorktreeRunner preloads feature/auth (dirty+behind), bugfix/login, chore/logout.
func threeWorktreeRunner() *mockRunner {
	porcelain := porcelainEntries(
		struct{ path, branch string }{pathWtFeatureAuth, branchFeatureAuth},
		struct{ path, branch string }{pathWtBugfixLogin, branchBugfixLogin},
		struct{ path, branch string }{pathWtChoreLogout, branchChoreLogout},
	)
	return &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList {
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: func(dir string, args ...string) (string, error) {
			if dir != pathWtFeatureAuth {
				return "", nil
			}
			if len(args) >= 1 && args[0] == gitCmdStatus {
				return dirtyStatusFixture, nil
			}
			if len(args) >= 1 && args[0] == "rev-list" {
				return "2\t0", nil // behind=2, ahead=0
			}
			return "", nil
		},
	}
}

// mockGHRunner routes "auth status" to authErr and "pr list --head <branch>" to prByBranch.
type mockGHRunner struct {
	authErr    error
	prByBranch map[string]string // branch → raw gh pr list JSON or "" for network err
	prErr      error             // if non-nil, every pr query fails
}

func (m *mockGHRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
		return nil, m.authErr
	}
	if len(args) >= 2 && args[0] == "pr" && args[1] == "list" {
		if m.prErr != nil {
			return nil, m.prErr
		}
		for i, a := range args {
			if a == "--head" && i+1 < len(args) {
				branch := args[i+1]
				if out, ok := m.prByBranch[branch]; ok {
					return []byte(out), nil
				}
				return []byte(prListNoOpenPR), nil
			}
		}
	}
	return nil, nil
}

// withFakeGhOnPath puts a dummy gh binary on PATH so gh.IsAvailable()
// returns true. The runner is always mocked so the binary is never executed.
func withFakeGhOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	fake := filepath.Join(dir, "gh")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestListWorktreesNoFiltersNoFull(t *testing.T) {
	gitR := threeWorktreeRunner()
	res, err := ListWorktrees(context.Background(), gitR, nil, ListWorktreesRequest{
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(res.Rows))
	}
	if res.PRInfos != nil {
		t.Error("PRInfos should be nil when Full=false")
	}
	if res.GhWarning != "" {
		t.Errorf("GhWarning = %q, want empty", res.GhWarning)
	}
	wantOrder := []string{"auth", "login", "logout"}
	for i, want := range wantOrder {
		if res.Rows[i].Task != want {
			t.Errorf("Rows[%d].Task = %q, want %q", i, res.Rows[i].Task, want)
		}
	}
}

func TestListWorktreesFilter(t *testing.T) {
	tests := []struct {
		name       string
		req        ListWorktreesRequest
		wantBranch string
	}{
		{
			name:       "type filter",
			req:        ListWorktreesRequest{TypeFilter: "bugfix", WorktreeDir: wtDirTest},
			wantBranch: branchBugfixLogin,
		},
		{
			name:       "dirty filter",
			req:        ListWorktreesRequest{Dirty: true, WorktreeDir: wtDirTest},
			wantBranch: branchFeatureAuth,
		},
		{
			name:       "behind filter",
			req:        ListWorktreesRequest{Behind: true, WorktreeDir: wtDirTest},
			wantBranch: branchFeatureAuth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ListWorktrees(context.Background(), threeWorktreeRunner(), nil, tt.req)
			if err != nil {
				t.Fatalf("ListWorktrees: %v", err)
			}
			if len(res.Rows) != 1 || res.Rows[0].Branch != tt.wantBranch {
				t.Fatalf("want 1 row with branch %q, got %+v", tt.wantBranch, res.Rows)
			}
		})
	}
}

func TestListWorktreesServiceFilter(t *testing.T) {
	porcelain := porcelainEntries(
		struct{ path, branch string }{"/wt/auth-api-feature-login", "auth-api/feature/login"},
		struct{ path, branch string }{"/wt/web-app-feature-login", "web-app/feature/login"},
	)
	gitR := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList {
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	res, err := ListWorktrees(context.Background(), gitR, nil, ListWorktreesRequest{
		Service:     "auth-api",
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0].Service != "auth-api" {
		t.Fatalf("want 1 auth-api row, got %+v", res.Rows)
	}
}

func TestListWorktreesFullGhUnauth(t *testing.T) {
	withFakeGhOnPath(t)
	gitR := threeWorktreeRunner()
	ghR := &mockGHRunner{authErr: errors.New("unauth")}

	res, err := ListWorktrees(context.Background(), gitR, ghR, ListWorktreesRequest{
		Full:        true,
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if res.GhWarning != GhUnavailableWarning {
		t.Errorf("GhWarning = %q, want %q", res.GhWarning, GhUnavailableWarning)
	}
	if res.PRInfos != nil {
		t.Error("PRInfos should be nil when gh auth fails")
	}
	if len(res.Rows) != 3 {
		t.Errorf("got %d rows, want 3", len(res.Rows))
	}
}

func TestListWorktreesFullGhNoPR(t *testing.T) {
	withFakeGhOnPath(t)
	gitR := threeWorktreeRunner()
	ghR := &mockGHRunner{prByBranch: map[string]string{}}

	res, err := ListWorktrees(context.Background(), gitR, ghR, ListWorktreesRequest{
		Full:        true,
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if res.GhWarning != "" {
		t.Errorf("GhWarning = %q, want empty", res.GhWarning)
	}
	if res.PRInfos == nil {
		t.Fatal("PRInfos should be non-nil when gh auth succeeds")
	}
	for _, row := range res.Rows {
		info := res.PRInfos[row.Branch]
		if info.Number != 0 || info.CIStatus != "" {
			t.Errorf("branch %q: got %+v, want zero PRInfo", row.Branch, info)
		}
	}
}

func TestListWorktreesFullPRRollupStates(t *testing.T) {
	withFakeGhOnPath(t)

	cases := []struct {
		name       string
		prJSON     string
		wantNumber int
		wantCI     gh.CIStatus
	}{
		{"success", prListEntrySuccess, 12, gh.CIStatusSuccess},
		{"pending", prListEntryPending, 13, gh.CIStatusPending},
		{"failure", prListEntryFailure, 14, gh.CIStatusFailure},
		{"no-checks", prListEntryNoCheck, 15, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gitR := threeWorktreeRunner()
			ghR := &mockGHRunner{
				prByBranch: map[string]string{branchFeatureAuth: tc.prJSON},
			}
			res, err := ListWorktrees(context.Background(), gitR, ghR, ListWorktreesRequest{
				Full:        true,
				WorktreeDir: wtDirTest,
			})
			if err != nil {
				t.Fatalf("ListWorktrees: %v", err)
			}
			info := res.PRInfos[branchFeatureAuth]
			if info.Number != tc.wantNumber {
				t.Errorf("Number = %d, want %d", info.Number, tc.wantNumber)
			}
			if info.CIStatus != tc.wantCI {
				t.Errorf("CIStatus = %q, want %q", info.CIStatus, tc.wantCI)
			}
		})
	}
}

func TestListWorktreesFullComposesWithDirtyFilter(t *testing.T) {
	withFakeGhOnPath(t)
	gitR := threeWorktreeRunner()
	ghR := &mockGHRunner{
		prByBranch: map[string]string{branchFeatureAuth: prListEntrySuccess},
	}

	res, err := ListWorktrees(context.Background(), gitR, ghR, ListWorktreesRequest{
		Full:        true,
		Dirty:       true,
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0].Branch != branchFeatureAuth {
		t.Fatalf("want 1 dirty row (feature/auth), got %+v", res.Rows)
	}
	// PRInfos carries entries for all candidates queried before the status filter runs.
	if got := res.PRInfos[branchFeatureAuth]; got.Number != 12 || got.CIStatus != gh.CIStatusSuccess {
		t.Errorf("PRInfos[feature/auth] = %+v, want {12, SUCCESS}", got)
	}
}

func TestListWorktreesFullPRQueryErrorDegradesSilently(t *testing.T) {
	withFakeGhOnPath(t)
	gitR := threeWorktreeRunner()
	ghR := &mockGHRunner{prErr: errors.New("network down")}

	res, err := ListWorktrees(context.Background(), gitR, ghR, ListWorktreesRequest{
		Full:        true,
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	// Every branch ends up with a zero PRInfo (silent degradation).
	if res.PRInfos == nil {
		t.Fatal("PRInfos should be non-nil; auth succeeded")
	}
	for _, row := range res.Rows {
		if got := res.PRInfos[row.Branch]; got.Number != 0 {
			t.Errorf("branch %q: got Number=%d, want 0 under query error", row.Branch, got.Number)
		}
	}
}

func TestListWorktreesFullWithNilGhRunnerSkipsQueries(t *testing.T) {
	gitR := threeWorktreeRunner()
	res, err := ListWorktrees(context.Background(), gitR, nil, ListWorktreesRequest{
		Full:        true,
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if res.PRInfos != nil {
		t.Error("PRInfos should be nil when ghR is nil")
	}
	if res.GhWarning != "" {
		t.Errorf("GhWarning = %q, want empty (no gh attempted)", res.GhWarning)
	}
}

func TestListWorktreesCurrentPathMarksRow(t *testing.T) {
	// Real tmp dir needed so EvalSymlinks resolves to the path we compare against.
	dir := t.TempDir()
	wtPath := filepath.Join(dir, "feature-auth")
	if err := os.Mkdir(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	porcelain := porcelainEntries(
		struct{ path, branch string }{wtPath, branchFeatureAuth},
		struct{ path, branch string }{filepath.Join(dir, "bugfix-login"), branchBugfixLogin},
	)
	if err := os.Mkdir(filepath.Join(dir, "bugfix-login"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitR := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList {
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	res, err := ListWorktrees(context.Background(), gitR, nil, ListWorktreesRequest{
		CurrentPath: wtPath,
		WorktreeDir: dir,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}

	var currentCount int
	for _, row := range res.Rows {
		if row.IsCurrent {
			currentCount++
			if row.Branch != branchFeatureAuth {
				t.Errorf("IsCurrent on %q, want on %q", row.Branch, branchFeatureAuth)
			}
		}
	}
	if currentCount != 1 {
		t.Errorf("IsCurrent rows = %d, want 1", currentCount)
	}
}

func TestListWorktreesGitListError(t *testing.T) {
	gitR := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errGitFailed },
		runInDir: noopRunInDir,
	}
	_, err := ListWorktrees(context.Background(), gitR, nil, ListWorktreesRequest{})
	if err == nil {
		t.Fatal("expected error from git.ListWorktrees")
	}
}

func TestListWorktreesSkipsBareEntries(t *testing.T) {
	porcelain := cmdWorktreeTest + " /repo/.git/worktrees/bare\nHEAD abc\nbare\n\n" +
		cmdWorktreeTest + " " + pathWtFeatureAuth + "\nHEAD def\nbranch refs/heads/" + branchFeatureAuth + "\n"
	gitR := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[0] == cmdWorktreeTest && args[1] == cmdList {
				return porcelain, nil
			}
			return "", nil
		},
		runInDir: noopRunInDir,
	}
	res, err := ListWorktrees(context.Background(), gitR, nil, ListWorktreesRequest{
		WorktreeDir: wtDirTest,
	})
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("got %d rows (bare should be skipped), want 1", len(res.Rows))
	}
}
