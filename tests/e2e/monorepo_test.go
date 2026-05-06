package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

// setupMonorepoRepo creates an initialized repo with a service subdirectory.
func setupMonorepoRepo(t *testing.T, services ...string) string {
	t.Helper()
	repo := setupInitializedRepo(t)
	for _, svc := range services {
		if err := os.Mkdir(filepath.Join(repo, svc), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestMonorepoAddCreatesWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	r := rimbaSuccess(t, repo, "add", "auth-api/my-task")
	assertContains(t, r.Stdout, msgCreatedWorktree)
	assertContains(t, r.Stdout, "service: auth-api")

	// Verify branch name includes service
	assertContains(t, r.Stdout, "auth-api/feature/my-task")

	// Verify worktree directory exists
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.FullBranchName("auth-api", defaultPrefix, "my-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, wtPath)
}

func TestMonorepoAddWithPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	r := rimbaSuccess(t, repo, "add", "--bugfix", "auth-api/crash-fix")
	assertContains(t, r.Stdout, "auth-api/bugfix/crash-fix")
}

func TestMonorepoAddMultiSegmentTask(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	r := rimbaSuccess(t, repo, "add", "auth-api/auth-redirect/part-1")
	assertContains(t, r.Stdout, "auth-api/feature/auth-redirect-part-1")
}

func TestNonMonorepoFallback(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// "no-such-dir" doesn't exist, so treated as standard with sanitization
	r := rimbaSuccess(t, repo, "add", "no-such-dir/my-task")
	assertContains(t, r.Stdout, "feature/no-such-dir-my-task")
}

func TestStandardAddUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "add", "simple-task")
	assertContains(t, r.Stdout, "feature/simple-task")
	assertNotContains(t, r.Stdout, "service:")
}

func TestMonorepoRemove(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	rimbaSuccess(t, repo, "add", "auth-api/my-task")
	r := rimbaSuccess(t, repo, "remove", "auth-api/my-task")
	assertContains(t, r.Stdout, "Removed worktree")
}

func TestMonorepoListShowsServiceColumn(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	rimbaSuccess(t, repo, "add", "auth-api/my-task")
	r := rimbaSuccess(t, repo, "list")
	assertContains(t, r.Stdout, "SERVICE")
	assertContains(t, r.Stdout, "auth-api")
}

func TestMonorepoListServiceFilter(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api", "web-app")

	rimbaSuccess(t, repo, "add", "auth-api/task-a")
	rimbaSuccess(t, repo, "add", "web-app/task-b")

	// Filter by auth-api
	r := rimbaSuccess(t, repo, "list", "--service", "auth-api")
	assertContains(t, r.Stdout, "task-a")
	assertNotContains(t, r.Stdout, "task-b")
}

func TestStandardListNoServiceColumn(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	rimbaSuccess(t, repo, "add", "plain-task")
	r := rimbaSuccess(t, repo, "list")
	assertNotContains(t, r.Stdout, "SERVICE")
}

func TestMonorepoDuplicateStripsServicePrefix(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	rimbaSuccess(t, repo, "add", "auth-api/login")
	r := rimbaSuccess(t, repo, "duplicate", "auth-api/login")

	assertContains(t, r.Stdout, "auth-api/feature/login-1")
	assertNotContains(t, r.Stdout, "auth-api/feature/auth-api/login-1")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.FullBranchName("auth-api", defaultPrefix, "login-1")
	assertFileExists(t, resolver.WorktreePath(wtDir, branch))
}

func TestMonorepoRenameStripsServicePrefix(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	rimbaSuccess(t, repo, "add", "auth-api/old")
	r := rimbaSuccess(t, repo, "rename", "auth-api/old", "auth-api/new")

	assertContains(t, r.Stdout, "Renamed worktree")
	assertNotContains(t, r.Stdout, "auth-api/feature/auth-api/new")
	assertNotContains(t, r.Stdout, "auth-api/old")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.FullBranchName("auth-api", defaultPrefix, "new")
	assertFileExists(t, resolver.WorktreePath(wtDir, branch))
}

func TestMonorepoRenameBareNewTask(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	rimbaSuccess(t, repo, "add", "auth-api/old2")
	r := rimbaSuccess(t, repo, "rename", "auth-api/old2", "new2")

	assertContains(t, r.Stdout, "Renamed worktree")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.FullBranchName("auth-api", defaultPrefix, "new2")
	assertFileExists(t, resolver.WorktreePath(wtDir, branch))
}

func TestMonorepoRestoreFindsArchivedBranch(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupMonorepoRepo(t, "auth-api")

	rimbaSuccess(t, repo, "add", "auth-api/login")
	rimbaSuccess(t, repo, "archive", "auth-api/login")
	r := rimbaSuccess(t, repo, "restore", "auth-api/login", flagSkipDepsE2E, flagSkipHooksE2E)

	assertContains(t, r.Stdout, "Restored worktree for task")
	assertContains(t, r.Stdout, "auth-api/feature/login")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.FullBranchName("auth-api", defaultPrefix, "login")
	assertFileExists(t, resolver.WorktreePath(wtDir, branch))
}
