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
	testutil.GitCmd(t, repo, "branch", flagBranchList, branch)
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

	// Create a file with no read permissions so copyFile fails on os.Open
	envPath := filepath.Join(repo, ".env")
	if err := os.WriteFile(envPath, []byte("SECRET=fail"), 0000); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Restore permissions on cleanup so t.TempDir() can remove the file
	t.Cleanup(func() { _ = os.Chmod(envPath, 0644) })

	rimbaSuccess(t, repo, "init")

	// Override copy_files to include the unreadable file
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{".env"}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatalf(msgSaveConfig, err)
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

func TestAddCopiesDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create .vscode/settings.json before init
	vscodeDir := filepath.Join(repo, dotVscode)
	if err := os.Mkdir(vscodeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testutil.CreateFile(t, vscodeDir, settingsJSON, `{"go.formatTool":"goimports"}`)

	rimbaSuccess(t, repo, "init")

	// Override copy_files to include the directory
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{dotVscode}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatalf(msgSaveConfig, err)
	}

	r := rimbaSuccess(t, repo, "add", "with-dir")
	assertContains(t, r.Stdout, dotVscode)

	// Verify directory was copied to worktree
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "with-dir")
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, filepath.Join(wtPath, dotVscode, settingsJSON))

	data, err := os.ReadFile(filepath.Join(wtPath, dotVscode, settingsJSON))
	if err != nil {
		t.Fatalf("failed to read copied settings.json: %v", err)
	}
	if string(data) != `{"go.formatTool":"goimports"}` {
		t.Errorf("expected settings.json content, got %q", data)
	}
}

func TestAddCopiesNestedDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create .config/sub/settings.toml before init
	subDir := filepath.Join(repo, dotConfig, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	testutil.CreateFile(t, subDir, "settings.toml", "key = \"value\"")

	rimbaSuccess(t, repo, "init")

	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{dotConfig}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatalf(msgSaveConfig, err)
	}

	rimbaSuccess(t, repo, "add", "with-nested")

	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "with-nested")
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, filepath.Join(wtPath, dotConfig, "sub", "settings.toml"))
}

func TestAddSkipsMissingDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)
	rimbaSuccess(t, repo, "init")

	// Override copy_files to include a non-existent directory
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{".nonexistent-dir"}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatalf(msgSaveConfig, err)
	}

	// Should succeed — missing entries are silently skipped
	rimbaSuccess(t, repo, "add", "skip-missing-dir")
}

func TestAddShowsHints(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "add", "hint-task")
	assertContains(t, r.Stderr, "Options:")
	assertContains(t, r.Stderr, flagSkipDepsE2E)
	assertContains(t, r.Stderr, flagSkipHooksE2E)
	assertContains(t, r.Stderr, "--source")
}

func TestAddHintsFilterUsedFlags(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "add", flagSkipDepsE2E, flagSkipHooksE2E, "hint-filter-task")
	assertNotContains(t, r.Stderr, flagSkipDepsE2E)
	assertNotContains(t, r.Stderr, flagSkipHooksE2E)
	// source flag was not used, so it should still appear
	assertContains(t, r.Stderr, "--source")
}

func TestAddHintsSuppressedByRIMBAQUIET(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaWithEnv(t, repo, []string{"RIMBA_QUIET=1"}, "add", "hint-quiet-task")
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}
	assertNotContains(t, r.Stderr, "Options:")
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
