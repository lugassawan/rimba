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
	saveConfig(t, repo, cfg)

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
	saveConfig(t, repo, cfg)

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
	saveConfig(t, repo, cfg)

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
	saveConfig(t, repo, cfg)

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

func TestAddCopiesFromMainRepoRoot(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create a gitignored file that only exists at the main repo root
	testutil.CreateFile(t, repo, ".claude", "claude-config")

	rimbaSuccess(t, repo, "init")

	// Override copy_files to include .claude
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{".claude"}
	saveConfig(t, repo, cfg)

	// Create a first worktree
	rimbaSuccess(t, repo, "add", "first-wt")

	// Now run rimba add FROM the first worktree — this is the key scenario.
	// RepoRoot would return the worktree path, but MainRepoRoot returns the main repo.
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	firstBranch := resolver.BranchName(defaultPrefix, "first-wt")
	firstWtPath := resolver.WorktreePath(wtDir, firstBranch)

	r := rimbaSuccess(t, firstWtPath, "add", "second-wt")
	assertContains(t, r.Stdout, "Created worktree")

	// Verify .claude was copied to the second worktree
	secondBranch := resolver.BranchName(defaultPrefix, "second-wt")
	secondWtPath := resolver.WorktreePath(wtDir, secondBranch)
	assertFileExists(t, filepath.Join(secondWtPath, ".claude"))

	data, err := os.ReadFile(filepath.Join(secondWtPath, ".claude"))
	if err != nil {
		t.Fatalf("failed to read copied .claude: %v", err)
	}
	if string(data) != "claude-config" {
		t.Errorf("expected .claude content %q, got %q", "claude-config", string(data))
	}
}

func TestAddCopiesNestedFile(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create a nested file in the repo root
	subDir := filepath.Join(repo, dotVscode)
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testutil.CreateFile(t, subDir, "settings.local.json", `{"local":true}`)

	rimbaSuccess(t, repo, "init")

	// Override copy_files to include the nested file path (not directory)
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{dotVscode + "/settings.local.json"}
	saveConfig(t, repo, cfg)

	rimbaSuccess(t, repo, "add", "nested-file-task")

	// Verify the nested file was copied with parent dir created
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "nested-file-task")
	wtPath := resolver.WorktreePath(wtDir, branch)
	assertFileExists(t, filepath.Join(wtPath, dotVscode, "settings.local.json"))

	data, err := os.ReadFile(filepath.Join(wtPath, dotVscode, "settings.local.json"))
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(data) != `{"local":true}` {
		t.Errorf("expected content %q, got %q", `{"local":true}`, string(data))
	}
}

func TestAddShowsSkippedFiles(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create only one of the listed files
	testutil.CreateFile(t, repo, ".env", "SECRET=value")

	rimbaSuccess(t, repo, "init")

	// Override copy_files to include both existing and non-existing entries
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{".env", ".nonexistent", ".also-missing"}
	saveConfig(t, repo, cfg)

	r := rimbaSuccess(t, repo, "add", "skipped-test")
	assertContains(t, r.Stdout, "Copied: [.env]")
	assertContains(t, r.Stdout, "Skipped (not found): [.nonexistent .also-missing]")
}

// loadConfig is a test helper that loads the rimba config from a repo.
// It calls FillDefaults to auto-derive missing fields (worktree_dir, default_source).
func loadConfig(t *testing.T, repo string) *config.Config {
	t.Helper()
	cfg, err := config.Resolve(repo)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg.FillDefaults(filepath.Base(repo), branchMain)
	return cfg
}

// saveConfig is a test helper that saves config to the .rimba/ directory.
func saveConfig(t *testing.T, repo string, cfg *config.Config) {
	t.Helper()
	dir := filepath.Join(repo, configDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir .rimba: %v", err)
	}
	if err := config.Save(filepath.Join(dir, teamFile), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}
