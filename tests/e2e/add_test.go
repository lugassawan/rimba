package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestAddCreatesWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "add", "my-task")
	assertContains(t, r.Stdout, msgCreatedWorktree)

	// Verify worktree directory exists
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "my-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, wtPath)

	// Verify branch exists
	testutil.GitCmd(t, repo, "branch", "--list", branch)
}

func TestAddCustomPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "add", "--bugfix", taskFix123)
	assertContains(t, r.Stdout, msgCreatedWorktree)
	assertContains(t, r.Stdout, bugfixPrefix+taskFix123)

	// Verify the worktree dir name uses the bugfix prefix
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(bugfixPrefix, taskFix123)
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, wtPath)
}

func TestAddMutuallyExclusiveFlags(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaFail(t, repo, "add", "--bugfix", "--hotfix", "oops")
	assertContains(t, r.Stderr, "none of the others can be")
}

func TestAddCustomSource(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create a develop branch with unique content before init
	testutil.GitCmd(t, repo, "checkout", "-b", "develop")
	testutil.CreateFile(t, repo, "develop.txt", "develop content")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "develop commit")
	testutil.GitCmd(t, repo, "checkout", "main")

	rimbaSuccess(t, repo, "init")

	r := rimbaSuccess(t, repo, "add", "-s", "develop", "from-develop")
	assertContains(t, r.Stdout, msgCreatedWorktree)

	// Verify the worktree was created from develop (develop.txt should exist)
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "from-develop")
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, filepath.Join(wtPath, "develop.txt"))
}

func TestAddCopiesDotfiles(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create .env before init
	envContent := "SECRET=value123"
	testutil.CreateFile(t, repo, ".env", envContent)

	rimbaSuccess(t, repo, "init")
	rimbaSuccess(t, repo, "add", "with-dotfiles")

	// Verify .env was copied to worktree
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "with-dotfiles")
	wtPath := resolver.WorktreePath(wtDir, branch)

	copiedEnv := filepath.Join(wtPath, ".env")
	assertFileExists(t, copiedEnv)

	data, err := os.ReadFile(copiedEnv)
	if err != nil {
		t.Fatalf("failed to read copied .env: %v", err)
	}
	if string(data) != envContent {
		t.Errorf("expected .env content %q, got %q", envContent, string(data))
	}
}

func TestAddSkipsMissingDotfiles(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// No dotfiles exist, but add should still succeed
	rimbaSuccess(t, repo, "add", "no-dotfiles")
}

func TestAddFailsWithoutInit(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	r := rimbaFail(t, repo, "add", task1)
	assertContains(t, r.Stderr, "rimba init")
}

func TestAddFailsDuplicate(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", "dup-task")

	r := rimbaFail(t, repo, "add", "dup-task")
	assertContains(t, r.Stderr, "already exists")
}

func TestAddPartialFailCopyHint(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create a directory that will trigger EISDIR when copyFile tries io.Copy
	if err := os.MkdirAll(filepath.Join(repo, ".testdir"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	rimbaSuccess(t, repo, "init")

	// Override copy_files to include the directory
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{".testdir"}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	r := rimbaFail(t, repo, "add", "copy-fail-task")
	assertContains(t, r.Stderr, "worktree created but failed to copy files")
	assertContains(t, r.Stderr, "To retry, manually copy files to:")
	assertContains(t, r.Stderr, "rimba remove copy-fail-task")
}

func TestAddFailsNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaFail(t, repo, "add")
}

// loadConfig is a test helper that loads the rimba config from a repo.
func loadConfig(t *testing.T, repo string) *config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join(repo, configFile))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	return cfg
}
