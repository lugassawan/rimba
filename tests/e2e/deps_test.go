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

// envForceCowEligible pins cowEligible's decision to true for a rimba
// invocation, so tests asserting clone-from-sibling behavior for
// install-capable modules (pnpm/yarn/npm node_modules) don't depend on the
// test host's real filesystem CoW support — CI runners' temp filesystems
// commonly don't support reflink at all, which correctly (and intentionally)
// routes such modules to their install command instead.
var envForceCowEligible = []string{"RIMBA_COW_ELIGIBLE_OVERRIDE=1"}

// rimbaSuccessWithEnv is rimbaSuccess with extra environment variables.
func rimbaSuccessWithEnv(t *testing.T, dir string, extraEnv []string, args ...string) result {
	t.Helper()
	r := rimbaWithEnv(t, dir, extraEnv, args...)
	if r.ExitCode != 0 {
		t.Fatalf("rimba %v: expected exit 0, got %d\nstdout: %s\nstderr: %s",
			args, r.ExitCode, r.Stdout, r.Stderr)
	}
	return r
}

func commitLockfile(t *testing.T, repo, name string) {
	t.Helper()
	testutil.CreateFile(t, repo, name, testLockContent)
	testutil.GitCmd(t, repo, "add", name)
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
	commitLockfile(t, repo, deps.LockfilePnpm)

	// Create first worktree
	rimbaSuccess(t, repo, "add", "task-deps-1")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "task-deps-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	// Manually create node_modules in the first worktree (simulating npm install)
	addNodeModules(t, wt1Path, map[string]string{".package-lock.json": "{}"})

	// Create second worktree — should clone node_modules
	r := rimbaSuccessWithEnv(t, repo, envForceCowEligible, "add", "task-deps-2")

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

	r := rimbaSuccessWithEnv(t, repo, envForceCowEligible, "add", "mono-2")

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
	commitLockfile(t, repo, deps.LockfilePnpm)

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
	commitLockfile(t, repo, deps.LockfilePnpm)

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
	commitLockfile(t, repo, deps.LockfilePnpm)

	rimbaSuccess(t, repo, "add", "install-src")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "install-src")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	addNodeModules(t, wt1Path, map[string]string{testPkgJSON: "{}"})

	// Create second worktree with --skip-deps (no deps initially)
	rimbaSuccess(t, repo, "add", flagSkipDepsE2E, taskInstallDst)

	r := rimbaSuccessWithEnv(t, repo, envForceCowEligible, "deps", "install", taskInstallDst)

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
	saveConfig(t, repo, cfg)

	r := rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "add", "hook-task")
	if r.ExitCode != 0 {
		t.Fatalf("rimba add hook-task: exit %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

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
	saveConfig(t, repo, cfg)

	r := rimbaWithEnv(t, repo, []string{"RIMBA_TRUST_YES=1"}, "add", flagSkipHooksE2E, "skip-hook-task")
	if r.ExitCode != 0 {
		t.Fatalf("rimba add --skip-hooks: exit %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}

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
	commitLockfile(t, repo, deps.LockfilePnpm)

	rimbaSuccess(t, repo, "add", taskDupSrc)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	srcBranch := resolver.BranchName(defaultPrefix, taskDupSrc)
	srcPath := resolver.WorktreePath(wtDir, srcBranch)

	if err := os.MkdirAll(filepath.Join(srcPath, deps.DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateFile(t, filepath.Join(srcPath, deps.DirNodeModules), "source.json", "from-source")

	r := rimbaSuccessWithEnv(t, repo, envForceCowEligible, "duplicate", taskDupSrc)

	dupBranch := resolver.BranchName(defaultPrefix, "dup-src-1")
	dupPath := resolver.WorktreePath(wtDir, dupBranch)

	assertFileExists(t, filepath.Join(dupPath, deps.DirNodeModules, "source.json"))
	assertContains(t, r.Stdout, msgDependencies)
	assertContains(t, r.Stdout, msgClonedFrom)
}

// TestAddWithDepsIneligibleCloneFallsBackToInstall verifies Stage 1's other
// headline case end-to-end: when cowEligible reports the destination
// filesystem can't honor a true reflink, an install-capable module runs its
// InstallCmd instead of byte-copying the sibling's node_modules. A config
// module override supplies a trivial InstallCmd so the assertion doesn't
// depend on pnpm being installed on the test host.
func TestAddWithDepsIneligibleCloneFallsBackToInstall(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo, deps.LockfilePnpm)

	cfg := loadConfig(t, repo)
	cfg.Deps = &config.DepsConfig{
		Modules: []config.ModuleConfig{
			{
				Dir:      deps.DirNodeModules,
				Lockfile: deps.LockfilePnpm,
				Install:  "mkdir -p node_modules && touch node_modules/installed.marker",
			},
		},
	}
	saveConfig(t, repo, cfg)
	testutil.GitCmd(t, repo, "add", ".")
	testutil.GitCmd(t, repo, "commit", "-m", "override node_modules install for e2e")

	rimbaSuccess(t, repo, "add", "--yes", "ineligible-src")

	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "ineligible-src")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	// A sibling that, if cloned, would be trivially distinguishable from a
	// fresh install (the install command only ever produces installed.marker).
	addNodeModules(t, wt1Path, map[string]string{"from-sibling.json": "{}"})

	r := rimbaSuccessWithEnv(t, repo, []string{"RIMBA_COW_ELIGIBLE_OVERRIDE=0"}, "add", "ineligible-dst")

	branch2 := resolver.BranchName(defaultPrefix, "ineligible-dst")
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	assertFileExists(t, filepath.Join(wt2Path, deps.DirNodeModules, "installed.marker"))
	assertFileNotExists(t, filepath.Join(wt2Path, deps.DirNodeModules, "from-sibling.json"))
	assertNotContains(t, r.Stdout, msgClonedFrom)
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
	commitLockfile(t, repo, deps.LockfilePnpm)

	cfg := loadConfig(t, repo)
	f := false
	cfg.Deps = &config.DepsConfig{AutoDetect: &f}
	saveConfig(t, repo, cfg)

	rimbaSuccess(t, repo, "add", "noauto-1")

	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "noauto-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	addNodeModules(t, wt1Path, map[string]string{testPkgJSON: "{}"})

	r := rimbaSuccess(t, repo, "add", "noauto-2")

	_ = strings.TrimSpace(r.Stdout)
	assertNotContains(t, r.Stdout, msgDependencies)
}

func TestAddWithDepsCloneRust(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo, deps.LockfileCargo)

	rimbaSuccess(t, repo, "add", "cargo-1")

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, "cargo-1")
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	targetDir := filepath.Join(wt1Path, deps.DirTarget)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	testutil.CreateFile(t, targetDir, "cargo-marker.txt", "built")

	r := rimbaSuccess(t, repo, "add", "cargo-2")

	branch2 := resolver.BranchName(defaultPrefix, "cargo-2")
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	assertFileExists(t, filepath.Join(wt2Path, deps.DirTarget, "cargo-marker.txt"))
	assertContains(t, r.Stdout, msgClonedFrom)
}

func TestAddWithDepsCloneVenv(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}
	assertVenvCloneRewritesPaths(t, deps.LockfileUv, "venv-1", "venv-2")
}

func TestAddWithDepsCloneVenvPoetry(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}
	assertVenvCloneRewritesPaths(t, deps.LockfilePoetry, "poetry-1", "poetry-2")
}

// assertVenvCloneRewritesPaths verifies that rimba clones .venv from wt1 to wt2 and
// rewrites baked absolute paths in bin/ scripts to point at the destination worktree.
func assertVenvCloneRewritesPaths(t *testing.T, lockfile, task1, task2 string) {
	t.Helper()

	repo := setupInitializedRepo(t)
	commitLockfile(t, repo, lockfile)

	rimbaSuccess(t, repo, "add", task1)

	cfg := loadConfig(t, repo)
	wtDir := filepath.Join(repo, cfg.WorktreeDir)
	branch1 := resolver.BranchName(defaultPrefix, task1)
	wt1Path := resolver.WorktreePath(wtDir, branch1)

	// Fabricate a .venv with a bin/ script that embeds wt1's absolute path.
	// Use the symlink-resolved path so the script matches what Python tools
	// actually embed (e.g. on macOS /tmp resolves to /private/tmp via git).
	realWt1Path := wt1Path
	if resolved, err := filepath.EvalSymlinks(wt1Path); err == nil {
		realWt1Path = resolved
	}
	venvBinDir := filepath.Join(wt1Path, deps.DirVenv, "bin")
	if err := os.MkdirAll(venvBinDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptContent := "#!" + filepath.Join(realWt1Path, deps.DirVenv) + "/bin/python3\nprint('hi')\n"
	scriptPath := filepath.Join(venvBinDir, "myapp")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	r := rimbaSuccess(t, repo, "add", task2)

	branch2 := resolver.BranchName(defaultPrefix, task2)
	wt2Path := resolver.WorktreePath(wtDir, branch2)

	clonedScript := filepath.Join(wt2Path, deps.DirVenv, "bin", "myapp")
	assertFileExists(t, clonedScript)

	// Path in cloned script must reference wt2, not wt1.
	// Use resolved paths to match what the rimba binary uses (git resolves symlinks).
	realWt2Path := wt2Path
	if resolved, err := filepath.EvalSymlinks(wt2Path); err == nil {
		realWt2Path = resolved
	}
	data, err := os.ReadFile(clonedScript)
	if err != nil {
		t.Fatal(err)
	}
	wt2Venv := filepath.Join(realWt2Path, deps.DirVenv)
	if !strings.Contains(string(data), wt2Venv) {
		t.Errorf("cloned script should contain wt2 venv path %q, got:\n%s", wt2Venv, data)
	}
	if strings.Contains(string(data), filepath.Join(realWt1Path, deps.DirVenv)) {
		t.Error("cloned script should NOT contain wt1 venv path")
	}
	assertContains(t, r.Stdout, msgClonedFrom)
}

func TestAddWithDepsCloneGradle(t *testing.T) {
	if testing.Short() {
		t.Skip(skipE2E)
	}

	cases := []struct {
		lockfile string
		task1    string
		task2    string
	}{
		{deps.LockfileGradle, "gradle-1", "gradle-2"},
		{deps.LockfileGradleKts, "gradle-kts-1", "gradle-kts-2"},
		{deps.LockfileGradleSettings, "gradle-settings-1", "gradle-settings-2"},
		{deps.LockfileGradleSettingsKts, "gradle-settings-kts-1", "gradle-settings-kts-2"},
	}

	for _, tc := range cases {
		t.Run(tc.lockfile, func(t *testing.T) {
			repo := setupInitializedRepo(t)
			commitLockfile(t, repo, tc.lockfile)

			rimbaSuccess(t, repo, "add", tc.task1)

			cfg := loadConfig(t, repo)
			wtDir := filepath.Join(repo, cfg.WorktreeDir)
			branch1 := resolver.BranchName(defaultPrefix, tc.task1)
			wt1Path := resolver.WorktreePath(wtDir, branch1)

			gradleDir := filepath.Join(wt1Path, deps.DirGradle)
			if err := os.MkdirAll(gradleDir, 0755); err != nil {
				t.Fatal(err)
			}
			testutil.CreateFile(t, gradleDir, "gradle-marker.txt", "cached")

			buildDir := filepath.Join(wt1Path, deps.DirGradleBuildOutput)
			if err := os.MkdirAll(buildDir, 0755); err != nil {
				t.Fatal(err)
			}
			testutil.CreateFile(t, buildDir, "build-marker.txt", "compiled")

			r := rimbaSuccess(t, repo, "add", tc.task2)

			branch2 := resolver.BranchName(defaultPrefix, tc.task2)
			wt2Path := resolver.WorktreePath(wtDir, branch2)

			assertFileExists(t, filepath.Join(wt2Path, deps.DirGradle, "gradle-marker.txt"))
			assertFileExists(t, filepath.Join(wt2Path, deps.DirGradleBuildOutput, "build-marker.txt"))
			assertContains(t, r.Stdout, msgClonedFrom)
		})
	}
}
