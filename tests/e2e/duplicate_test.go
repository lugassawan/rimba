package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

func TestDuplicateAutoSuffix(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Create source worktree with a file
	rimbaSuccess(t, repo, "add", taskDupA)
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	srcBranch := resolver.BranchName(defaultPrefix, taskDupA)
	srcPath := resolver.WorktreePath(wtDir, srcBranch)
	testutil.CreateFile(t, srcPath, "source.txt", "from "+taskDupA)
	testutil.GitCmd(t, srcPath, "add", ".")
	testutil.GitCmd(t, srcPath, "commit", "-m", "add source.txt")

	// Duplicate
	r := rimbaSuccess(t, repo, "duplicate", taskDupA)
	assertContains(t, r.Stdout, taskDupA+"-1")
	assertContains(t, r.Stdout, "Duplicated worktree")

	// Verify new worktree exists with source content
	dupBranch := resolver.BranchName(defaultPrefix, taskDupA+"-1")
	dupPath := resolver.WorktreePath(wtDir, dupBranch)
	assertFileExists(t, dupPath)
	assertFileExists(t, filepath.Join(dupPath, "source.txt"))
}

func TestDuplicateAutoSuffixIncrement(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaSuccess(t, repo, "add", taskDupB)

	// First duplicate
	r1 := rimbaSuccess(t, repo, "duplicate", taskDupB)
	assertContains(t, r1.Stdout, taskDupB+"-1")

	// Second duplicate
	r2 := rimbaSuccess(t, repo, "duplicate", taskDupB)
	assertContains(t, r2.Stdout, taskDupB+"-2")

	// Verify both exist
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	assertFileExists(t, resolver.WorktreePath(wtDir, resolver.BranchName(defaultPrefix, taskDupB+"-1")))
	assertFileExists(t, resolver.WorktreePath(wtDir, resolver.BranchName(defaultPrefix, taskDupB+"-2")))
}

func TestDuplicateWithAs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Create source with a file
	rimbaSuccess(t, repo, "add", "orig")
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	srcBranch := resolver.BranchName(defaultPrefix, "orig")
	srcPath := resolver.WorktreePath(wtDir, srcBranch)
	testutil.CreateFile(t, srcPath, "data.txt", "original data")
	testutil.GitCmd(t, srcPath, "add", ".")
	testutil.GitCmd(t, srcPath, "commit", "-m", "add data.txt")

	r := rimbaSuccess(t, repo, "duplicate", "orig", "--as", taskMyCopy)
	assertContains(t, r.Stdout, taskMyCopy)

	// Verify new worktree has source content
	dupBranch := resolver.BranchName(defaultPrefix, taskMyCopy)
	dupPath := resolver.WorktreePath(wtDir, dupBranch)
	assertFileExists(t, dupPath)
	assertFileExists(t, filepath.Join(dupPath, "data.txt"))
}

func TestDuplicateInheritsPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Create a bugfix worktree
	rimbaSuccess(t, repo, "add", "--bugfix", "fix-auth")

	r := rimbaSuccess(t, repo, "duplicate", "fix-auth")
	assertContains(t, r.Stdout, bugfixPrefix+"fix-auth-1")
}

func TestDuplicateCopiesDotfiles(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create .env before init
	testutil.CreateFile(t, repo, ".env", secretContent)

	rimbaSuccess(t, repo, "init")
	rimbaSuccess(t, repo, "add", "dot-src")

	r := rimbaSuccess(t, repo, "duplicate", "dot-src")
	assertContains(t, r.Stdout, "Copied")

	// Verify .env was copied
	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	dupBranch := resolver.BranchName(defaultPrefix, "dot-src-1")
	dupPath := resolver.WorktreePath(wtDir, dupBranch)
	copiedEnv := filepath.Join(dupPath, ".env")
	assertFileExists(t, copiedEnv)

	data, err := os.ReadFile(copiedEnv)
	if err != nil {
		t.Fatalf("failed to read copied .env: %v", err)
	}
	if string(data) != secretContent {
		t.Errorf("expected .env content %q, got %q", secretContent, string(data))
	}
}

func TestDuplicateCopiesDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupRepo(t)

	// Create .vscode/settings.json before init
	vscodeDir := filepath.Join(repo, dotVscode)
	if err := os.Mkdir(vscodeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testutil.CreateFile(t, vscodeDir, settingsJSON, `{"editor.fontSize":14}`)

	rimbaSuccess(t, repo, "init")

	// Override copy_files to include the directory
	cfg := loadConfig(t, repo)
	cfg.CopyFiles = []string{dotVscode}
	saveConfig(t, repo, cfg)

	rimbaSuccess(t, repo, "add", "dir-src")

	r := rimbaSuccess(t, repo, "duplicate", "dir-src")
	assertContains(t, r.Stdout, dotVscode)

	// Verify .vscode was copied to the duplicate worktree
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	dupBranch := resolver.BranchName(defaultPrefix, "dir-src-1")
	dupPath := resolver.WorktreePath(wtDir, dupBranch)
	assertFileExists(t, filepath.Join(dupPath, dotVscode, settingsJSON))

	data, err := os.ReadFile(filepath.Join(dupPath, dotVscode, settingsJSON))
	if err != nil {
		t.Fatalf("failed to read copied settings.json: %v", err)
	}
	if string(data) != `{"editor.fontSize":14}` {
		t.Errorf("expected settings.json content, got %q", data)
	}
}

func TestDuplicateSourceNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "duplicate", "nonexistent")
	assertContains(t, r.Stderr, "not found")
}

func TestDuplicateNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	rimbaFail(t, repo, "duplicate")
}

func TestDuplicateMain(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	r := rimbaFail(t, repo, "duplicate", "main")
	assertContains(t, r.Stderr, "cannot duplicate the default branch")
}
