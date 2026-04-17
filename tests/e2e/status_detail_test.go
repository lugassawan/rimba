package e2e_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

// recentCellRE matches the trailing 7D cell (digits or "?") on a status row.
var recentCellRE = regexp.MustCompile(`\s(\d+|\?)\s*$`)

func TestStatusDetailShowsSize(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	const task = "sizable"
	rimbaSuccess(t, repo, "add", task)

	wtPath := worktreePathFor(t, repo, task)
	// Write 5 KiB of data so the SIZE column renders as "5.0KB" (or above).
	if err := os.WriteFile(filepath.Join(wtPath, "blob.bin"), make([]byte, 5*1024), 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}

	r := rimbaSuccess(t, repo, "status", "--detail")
	assertContains(t, r.Stdout, "SIZE")
	assertContains(t, r.Stdout, "7D")
	assertContains(t, r.Stdout, "Disk:")
	// The written file is 5 KiB; the worktree's own git metadata pushes it
	// higher. Any KB/MB suffix in the SIZE column is acceptable evidence.
	if !strings.Contains(r.Stdout, "KB") && !strings.Contains(r.Stdout, "MB") {
		t.Errorf("expected SIZE column to contain KB or MB, got:\n%s", r.Stdout)
	}
}

func TestStatusDetailSortsBySize(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	const smallTask = "tiny-wt"
	const largeTask = "big-wt"
	rimbaSuccess(t, repo, "add", smallTask)
	rimbaSuccess(t, repo, "add", largeTask)

	// Inflate the "big" worktree by 2 MB so it clearly dwarfs the small one.
	bigPath := worktreePathFor(t, repo, largeTask)
	if err := os.WriteFile(filepath.Join(bigPath, "blob.bin"), make([]byte, 2*1024*1024), 0o644); err != nil {
		t.Fatalf("write big blob: %v", err)
	}

	r := rimbaSuccess(t, repo, "status", "--detail")
	bigIdx := strings.Index(r.Stdout, largeTask)
	smallIdx := strings.Index(r.Stdout, smallTask)
	if bigIdx < 0 || smallIdx < 0 {
		t.Fatalf("expected both tasks in output, got:\n%s", r.Stdout)
	}
	if bigIdx >= smallIdx {
		t.Errorf("expected %q to appear before %q under --detail (size desc), got:\n%s",
			largeTask, smallTask, r.Stdout)
	}
}

func TestStatusDetailFootprint(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "footprint-task")

	r := rimbaSuccess(t, repo, "status", "--detail")
	assertContains(t, r.Stdout, "Disk:")
	assertContains(t, r.Stdout, "main:")
	assertContains(t, r.Stdout, "worktrees:")
}

func TestStatusDetailVelocity(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	const task = "velocity-task"
	rimbaSuccess(t, repo, "add", task)

	wtPath := worktreePathFor(t, repo, task)
	// Add 3 commits in the worktree (recent). NewTestRepo's initial commit
	// is reachable from this branch too, so 7D >= 4. The precise value
	// depends on branch topology; asserting > 1 keeps the test focused
	// on "velocity reflects recent commits" without being brittle.
	for i := range 3 {
		testutil.CreateFile(t, wtPath, "v"+string(rune('a'+i))+".txt", "x")
		testutil.GitCmd(t, wtPath, "add", ".")
		testutil.GitCmd(t, wtPath, "commit", "-m", "v")
	}

	r := rimbaSuccess(t, repo, "status", "--detail")
	assertContains(t, r.Stdout, "7D")

	taskLine := lineContaining(t, r.Stdout, task)
	m := recentCellRE.FindStringSubmatch(taskLine)
	if m == nil {
		t.Fatalf("no 7D cell found at end of row; line: %q", taskLine)
	}
	recent := m[1]
	if recent == "?" {
		t.Fatalf("velocity for %q is '?', expected a number; line: %q", task, taskLine)
	}
	if recent == "0" || recent == "1" {
		t.Errorf("expected 7D > 1 after creating 3 commits, got %s (line: %q)", recent, taskLine)
	}
}

// worktreePathFor returns the path of the worktree created for task under
// the default prefix in the given rimba repo.
func worktreePathFor(t *testing.T, repo, task string) string {
	t.Helper()
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, task)
	return resolver.WorktreePath(wtDir, branch)
}

// lineContaining returns the first line of s that contains sub, or fails the test.
func lineContaining(t *testing.T, s, sub string) string {
	t.Helper()
	for line := range strings.SplitSeq(s, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	t.Fatalf("no line containing %q found in:\n%s", sub, s)
	return ""
}
