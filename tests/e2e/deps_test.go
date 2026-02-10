package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/testutil"
)

const (
	testLockContent = "lockfile-v6-content"
	commitAddLock   = "add lockfile"
	testDirAppWeb   = "app-web"
	msgDependencies = "Dependencies:"
	msgClonedFrom   = "cloned from"
	testPkgJSON     = "pkg.json"
	taskInstallDst  = "install-dst"
	taskDupSrc      = "dup-src"
)

func commitLockfile(t *testing.T, repo string) {
	t.Helper()
	testutil.CreateFile(t, repo, deps.LockfilePnpm, testLockContent)
	testutil.GitCmd(t, repo, "add", deps.LockfilePnpm)
	testutil.GitCmd(t, repo, "commit", "-m", commitAddLock)
}

func addNodeModules(t *testing.T, wtPath string, files map[string]string) {
	t.Helper()
	nmDir := filepath.Join(wtPath, deps.DirNodeModules)
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		testutil.CreateFile(t, nmDir, name, content)
	}
}

func TestAddWithDepsClone(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo)

	// Create first worktree
	rimbaSuccess(t, repo, "add", "task-deps-1")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "task-deps-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	// Manually create node_modules in the first worktree (simulating npm install)
	addNodeModules(t, wt1Path, map[string]string{".package-lock.json": "{}"})

	// Create second worktree — should clone node_modules
	r := rimbaSuccess(t, repo, "add", "task-deps-2")

	branch2 := resolver.BranchName(defaultPrefix, "task-deps-2")
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	assertFileExists(t, filepath.Join(wt2Path, deps.DirNodeModules, ".package-lock.json"))
	assertContains(t, r.Stdout, msgDependencies)
	assertContains(t, r.Stdout, deps.DirNodeModules)
	assertContains(t, r.Stdout, msgClonedFrom)
}

func TestAddWithDepsNestedModules(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Simulate monorepo: root lockfile + nested app
	testutil.CreateFile(t, repo, deps.LockfilePnpm, testLockContent)
	if err := os.MkdirAll(filepath.Join(repo, testDirAppWeb), 0755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateFile(t, filepath.Join(repo, testDirAppWeb), "index.js", "// app")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add lockfile and app")

	rimbaSuccess(t, repo, "add", "mono-1")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "mono-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	addNodeModules(t, wt1Path, map[string]string{"root.json": "{}"})

	if err := os.MkdirAll(filepath.Join(wt1Path, testDirAppWeb, deps.DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateFile(t, filepath.Join(wt1Path, testDirAppWeb, deps.DirNodeModules), "app.json", "{}")

	r := rimbaSuccess(t, repo, "add", "mono-2")

	branch2 := resolver.BranchName(defaultPrefix, "mono-2")
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	assertFileExists(t, filepath.Join(wt2Path, deps.DirNodeModules, "root.json"))
	assertFileExists(t, filepath.Join(wt2Path, testDirAppWeb, deps.DirNodeModules, "app.json"))
	assertContains(t, r.Stdout, msgClonedFrom)
}

func TestAddWithDepsNestedLockfile(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	// Simulate polyglot: nested Go project with vendor
	apiDir := "api"
	if err := os.MkdirAll(filepath.Join(repo, apiDir), 0755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateFile(t, filepath.Join(repo, apiDir), deps.LockfileGo, "hash123")
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "add go project")

	rimbaSuccess(t, repo, "add", "go-1")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "go-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	if err := os.MkdirAll(filepath.Join(wt1Path, apiDir, deps.DirVendor), 0755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateFile(t, filepath.Join(wt1Path, apiDir, deps.DirVendor), "modules.txt", "vendor")

	r := rimbaSuccess(t, repo, "add", "go-2")

	branch2 := resolver.BranchName(defaultPrefix, "go-2")
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	assertFileExists(t, filepath.Join(wt2Path, apiDir, deps.DirVendor, "modules.txt"))
	assertContains(t, r.Stdout, msgClonedFrom)
}

func TestAddWithDepsSkipFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo)

	rimbaSuccess(t, repo, "add", "skip-1")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "skip-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	addNodeModules(t, wt1Path, map[string]string{testPkgJSON: "{}"})

	// Create second worktree with --skip-deps
	r := rimbaSuccess(t, repo, "add", flagSkipDepsE2E, "skip-2")

	branch2 := resolver.BranchName(defaultPrefix, "skip-2")
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	assertFileNotExists(t, filepath.Join(wt2Path, deps.DirNodeModules))
	assertNotContains(t, r.Stdout, msgDependencies)
}

func TestAddWithDepsNoLockfile(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "add", "no-lock")

	assertNotContains(t, r.Stdout, msgDependencies)
}

func TestDepsStatus(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo)

	rimbaSuccess(t, repo, "add", "status-task")

	r := rimbaSuccess(t, repo, "deps", "status")
	assertContains(t, r.Stdout, deps.DirNodeModules)
	assertContains(t, r.Stdout, "main")
}

func TestDepsInstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo)

	rimbaSuccess(t, repo, "add", "install-src")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "install-src")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	addNodeModules(t, wt1Path, map[string]string{testPkgJSON: "{}"})

	// Create second worktree with --skip-deps (no deps initially)
	rimbaSuccess(t, repo, "add", flagSkipDepsE2E, taskInstallDst)

	r := rimbaSuccess(t, repo, "deps", "install", taskInstallDst)

	branch2 := resolver.BranchName(defaultPrefix, taskInstallDst)
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	assertFileExists(t, filepath.Join(wt2Path, deps.DirNodeModules, testPkgJSON))
	assertContains(t, r.Stdout, msgClonedFrom)
}

func TestPostCreateHooks(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	cfg := loadConfig(t, repo)
	cfg.PostCreate = []string{"touch hook-marker.txt"}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatal(err)
	}

	r := rimbaSuccess(t, repo, "add", "hook-task")

	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "hook-task")
	wtPath := resolver.WorktreePath(wtDir, branch)

	assertFileExists(t, filepath.Join(wtPath, "hook-marker.txt"))
	assertContains(t, r.Stdout, "Hooks:")
	assertContains(t, r.Stdout, "touch hook-marker.txt: ok")
}

func TestPostCreateHooksSkipFlag(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	cfg := loadConfig(t, repo)
	cfg.PostCreate = []string{"touch should-not-exist.txt"}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatal(err)
	}

	r := rimbaSuccess(t, repo, "add", flagSkipHooksE2E, "skip-hook-task")

	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch := resolver.BranchName(defaultPrefix, "skip-hook-task")
	wtPath := resolver.WorktreePath(wtDir, branch)

	assertFileNotExists(t, filepath.Join(wtPath, "should-not-exist.txt"))
	assertNotContains(t, r.Stdout, "Hooks:")
}

func TestDuplicateWithDepsClone(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo)

	rimbaSuccess(t, repo, "add", taskDupSrc)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	srcBranch := resolver.BranchName(defaultPrefix, taskDupSrc)
	srcPath := resolver.WorktreePath(wtDir, srcBranch)

	if err := os.MkdirAll(filepath.Join(srcPath, deps.DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateFile(t, filepath.Join(srcPath, deps.DirNodeModules), "source.json", "from-source")

	r := rimbaSuccess(t, repo, "duplicate", taskDupSrc)

	dupBranch := resolver.BranchName(defaultPrefix, "dup-src-1")
	dupPath := resolver.WorktreePath(wtDir, dupBranch)

	assertFileExists(t, filepath.Join(dupPath, deps.DirNodeModules, "source.json"))
	assertContains(t, r.Stdout, msgDependencies)
	assertContains(t, r.Stdout, msgClonedFrom)
}

func TestDepsStatusNoModules(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaSuccess(t, repo, "deps", "status")

	assertContains(t, r.Stdout, "no modules detected")
}

func TestDepsInstallNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)

	r := rimbaFail(t, repo, "deps", "install", "nonexistent")

	assertContains(t, r.Stderr, "worktree not found")
}

func TestAddWithDepsAutoDetectDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo)

	cfg := loadConfig(t, repo)
	f := false
	cfg.Deps = &config.DepsConfig{AutoDetect: &f}
	if err := config.Save(filepath.Join(repo, configFile), cfg); err != nil {
		t.Fatal(err)
	}

	rimbaSuccess(t, repo, "add", "noauto-1")

	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "noauto-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	addNodeModules(t, wt1Path, map[string]string{testPkgJSON: "{}"})

	r := rimbaSuccess(t, repo, "add", "noauto-2")

	_ = strings.TrimSpace(r.Stdout)
	assertNotContains(t, r.Stdout, msgDependencies)
}
