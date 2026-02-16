package deps

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	porcelainWorktree  = "worktree "
	fmtExpectedResults = "expected 1 result(s), got %d"

	fmtExpectedNoError   = "expected no error, got %v"
	fmtExpectedOneResult = "expected 1 result, got %d"
	testDirCustomDeps    = "custom/deps"
)

// mockRunner implements git.Runner for testing.
type mockRunner struct {
	worktreeOutput string
}

func (m *mockRunner) Run(args ...string) (string, error) {
	return m.worktreeOutput, nil
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	return m.Run(args...)
}

func mockWorktreeList(paths ...string) string {
	var b strings.Builder
	branches := []string{"refs/heads/main", "refs/heads/feature/task-1", "refs/heads/feature/other", "refs/heads/feature/new"}
	hashes := []string{"abc123", "def456", "ghi789", "jkl012"}
	for i, p := range paths {
		if i > 0 {
			b.WriteString("\n")
		}
		branch := branches[i%len(branches)]
		hash := hashes[i%len(hashes)]
		b.WriteString(porcelainWorktree + p + "\nHEAD " + hash + "\nbranch " + branch + "\n")
	}
	return b.String()
}

func TestManagerInstallClone(t *testing.T) {
	existingWT := t.TempDir()
	newWT := t.TempDir()

	writeFile(t, existingWT, LockfilePnpm, "lockfile-v6-content")
	writeFile(t, newWT, LockfilePnpm, "lockfile-v6-content")

	if err := os.MkdirAll(filepath.Join(existingWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existingWT, DirNodeModules), "package.json", "{}")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "pnpm install --frozen-lockfile",
			Recursive:  false,
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Cloned {
		t.Error("expected Cloned=true")
	}
	if r.Source != existingWT {
		t.Errorf("expected source %s, got %s", existingWT, r.Source)
	}
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}

	assertFileContent(t, filepath.Join(newWT, DirNodeModules, "package.json"), "{}")
}

func TestManagerInstallNoMatchCloneOnly(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfileGo, "go sum content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirVendor,
			Lockfile:   LockfileGo,
			InstallCmd: "go mod vendor",
			CloneOnly:  true,
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (no match)")
	}
	if r.Error != nil {
		t.Error("expected no error for CloneOnly skip")
	}
}

func TestManagerInstallNoLockfile(t *testing.T) {
	newWT := t.TempDir()

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:      DirNodeModules,
			Lockfile: LockfilePnpm,
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false")
	}
	if r.Error != nil {
		t.Error("expected no error for missing lockfile")
	}
}

func TestManagerInstallHashMismatch(t *testing.T) {
	existingWT := t.TempDir()
	newWT := t.TempDir()

	writeFile(t, existingWT, LockfilePnpm, "old content")
	writeFile(t, newWT, LockfilePnpm, valNewContent)

	if err := os.MkdirAll(filepath.Join(existingWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "echo skipped",
		},
	}

	results := mgr.Install(newWT, modules)

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (hash mismatch)")
	}
}

func TestManagerInstallPreferSource(t *testing.T) {
	sourceWT := t.TempDir()
	otherWT := t.TempDir()
	newWT := t.TempDir()

	lockContent := "same-lockfile-content"
	writeFile(t, sourceWT, LockfilePnpm, lockContent)
	writeFile(t, otherWT, LockfilePnpm, lockContent)
	writeFile(t, newWT, LockfilePnpm, lockContent)

	for _, dir := range []string{sourceWT, otherWT} {
		if err := os.MkdirAll(filepath.Join(dir, DirNodeModules), 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, DirNodeModules), "origin.txt", dir)
	}

	runner := &mockRunner{worktreeOutput: mockWorktreeList(sourceWT, otherWT, newWT)}

	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.InstallPreferSource(newWT, sourceWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Cloned {
		t.Fatal("expected Cloned=true")
	}
	if r.Source != sourceWT {
		t.Errorf("expected source to be sourceWT, got %s", r.Source)
	}

	assertFileContent(t, filepath.Join(newWT, DirNodeModules, "origin.txt"), sourceWT)
}

func TestResolveModulesAutoDetect(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfilePnpm, "lockfile-v6")

	wt1 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt1, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}

	modules, err := ResolveModules(dir, true, nil, []string{wt1})
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) == 0 {
		t.Fatal("expected at least 1 module from auto-detect")
	}
	if modules[0].Dir != DirNodeModules {
		t.Errorf("module.Dir = %q, want %q", modules[0].Dir, DirNodeModules)
	}
}

func TestResolveModulesConfigOnly(t *testing.T) {
	dir := t.TempDir()
	// No lockfiles present — auto-detect would find nothing

	configModules := []config.ModuleConfig{
		{Dir: testDirCustomDeps, Lockfile: "custom.lock", Install: "custom install"},
	}

	modules, err := ResolveModules(dir, false, configModules, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}
	if modules[0].Dir != testDirCustomDeps {
		t.Errorf("module.Dir = %q, want %q", modules[0].Dir, testDirCustomDeps)
	}
}

func TestResolveModulesEmpty(t *testing.T) {
	dir := t.TempDir()

	modules, err := ResolveModules(dir, true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if modules != nil {
		t.Errorf("expected nil modules, got %v", modules)
	}
}

func TestResolveModulesNoAutoDetectNoConfig(t *testing.T) {
	dir := t.TempDir()

	modules, err := ResolveModules(dir, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if modules != nil {
		t.Errorf("expected nil modules, got %v", modules)
	}
}

func TestResolveModulesFilterCloneOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfileGo, "go.sum content")

	// No existing worktrees have vendor/ → clone-only should be filtered out
	modules, err := ResolveModules(dir, true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The vendor module is CloneOnly and no worktree has it → filtered out
	for _, m := range modules {
		if m.Dir == DirVendor {
			t.Error("expected vendor module to be filtered out (CloneOnly, no existing worktrees)")
		}
	}
}

func TestManagerInstallHashError(t *testing.T) {
	newWT := t.TempDir()

	// Create a lockfile that can't be read
	lockPath := filepath.Join(newWT, LockfilePnpm)
	if err := os.WriteFile(lockPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make the lockfile unreadable
	if err := os.Chmod(lockPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockPath, 0644) })

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error for unreadable lockfile")
	}
}

func TestManagerInstallModuleNoHash(t *testing.T) {
	newWT := t.TempDir()
	// No lockfile — hash will be empty

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false when no lockfile")
	}
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}
}

func TestManagerInstallPreferSourceHashError(t *testing.T) {
	sourceWT := t.TempDir()
	newWT := t.TempDir()

	lockPath := filepath.Join(newWT, LockfilePnpm)
	if err := os.WriteFile(lockPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(lockPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockPath, 0644) })

	runner := &mockRunner{worktreeOutput: mockWorktreeList(sourceWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.InstallPreferSource(newWT, sourceWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error for unreadable lockfile")
	}
}

func TestManagerInstallFallbackToInstallCmd(t *testing.T) {
	existingWT := t.TempDir()
	newWT := t.TempDir()

	// Same lockfile in both, but different content — hash mismatch
	writeFile(t, existingWT, LockfilePnpm, "version-1")
	writeFile(t, newWT, LockfilePnpm, "version-2")

	// Existing has node_modules
	if err := os.MkdirAll(filepath.Join(existingWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "echo installed", // will actually try to run this
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	// Hash mismatch means no clone, falls through to InstallCmd
	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (hash mismatch)")
	}
}

func TestManagerInstallSingleWorktree(t *testing.T) {
	// Only one worktree (newWT itself) — existingPaths will be empty,
	// so no clone source. With a lockfile and InstallCmd, it falls through
	// to InstallCmd.
	newWT := t.TempDir()

	writeFile(t, newWT, LockfilePnpm, "lockfile-content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "echo ok",
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (no other worktree to clone from)")
	}
	// InstallCmd "echo ok" should succeed
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}
}

func TestManagerInstallCloneOnlyNoModDir(t *testing.T) {
	// Two worktrees with matching lockfile hash, but existingWT does NOT have
	// the module directory. CloneOnly module should skip without error.
	existingWT := t.TempDir()
	newWT := t.TempDir()

	lockContent := "go-sum-content"
	writeFile(t, existingWT, LockfileGo, lockContent)
	writeFile(t, newWT, LockfileGo, lockContent)

	// Note: existingWT does NOT have vendor/ directory

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:       DirVendor,
			Lockfile:  LockfileGo,
			CloneOnly: true,
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (no modDir in existing worktree)")
	}
	if r.Error != nil {
		t.Errorf("expected no error for CloneOnly skip, got %v", r.Error)
	}
}

func TestManagerInstallPreferSourceNoLockfile(t *testing.T) {
	sourceWT := t.TempDir()
	newWT := t.TempDir()

	// No lockfile in newWT — hash will be empty → Cloned=false, no error
	runner := &mockRunner{worktreeOutput: mockWorktreeList(sourceWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.InstallPreferSource(newWT, sourceWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (no lockfile, empty hash)")
	}
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}
}

func TestManagerInstallWithInstallCmd(t *testing.T) {
	// Lockfile present in newWT but no matching source worktree has same hash
	// or the module dir. Should fall through to InstallCmd.
	existingWT := t.TempDir()
	newWT := t.TempDir()

	writeFile(t, existingWT, LockfilePnpm, "different-content")
	writeFile(t, newWT, LockfilePnpm, "new-content")

	// existingWT has no node_modules at all

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "echo ok",
		},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (no matching source)")
	}
	// InstallCmd "echo ok" should succeed without error
	if r.Error != nil {
		t.Errorf("expected no error from InstallCmd, got %v", r.Error)
	}
}

// errorRunner is a mockRunner that always returns an error from Run.
type errorRunner struct {
	err error
}

func (e *errorRunner) Run(_ ...string) (string, error)              { return "", e.err }
func (e *errorRunner) RunInDir(_ string, _ ...string) (string, error) { return "", e.err }

var errGitFailed = errors.New("git worktree list failed")

func TestInstallListWorktreesError(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "content")

	mgr := &Manager{Runner: &errorRunner{err: errGitFailed}}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.Install(newWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error from ListWorktrees failure")
	}
	if !errors.Is(results[0].Error, errGitFailed) {
		t.Errorf("error = %v, want %v", results[0].Error, errGitFailed)
	}
}

func TestInstallPreferSourceListWorktreesError(t *testing.T) {
	newWT := t.TempDir()
	sourceWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "content")

	mgr := &Manager{Runner: &errorRunner{err: errGitFailed}}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.InstallPreferSource(newWT, sourceWT, modules)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error from ListWorktrees failure")
	}
	if !errors.Is(results[0].Error, errGitFailed) {
		t.Errorf("error = %v, want %v", results[0].Error, errGitFailed)
	}
}

func TestManagerInstallCloneFailFallback(t *testing.T) {
	existingWT := t.TempDir()

	lockContent := "same-lock-content"
	writeFile(t, existingWT, LockfilePnpm, lockContent)

	// Create module dir in existingWT so hash+stat pass
	if err := os.MkdirAll(filepath.Join(existingWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existingWT, DirNodeModules), "pkg.json", "{}")

	// newWT is inside a read-only parent so CloneModule cp can't write to it
	parentDir := t.TempDir()
	newWT := filepath.Join(parentDir, "new-wt")
	if err := os.MkdirAll(newWT, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, newWT, LockfilePnpm, lockContent)

	// Make newWT read-only so cp -R node_modules can't create destination
	if err := os.Chmod(newWT, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(newWT, 0755) })

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}

	// CloneOnly=true: clone fails → returns wrapped error
	t.Run("clone only error", func(t *testing.T) {
		modules := []Module{
			{Dir: DirNodeModules, Lockfile: LockfilePnpm, CloneOnly: true},
		}
		results := mgr.Install(newWT, modules)
		if len(results) != 1 {
			t.Fatalf(fmtExpectedOneResult, len(results))
		}
		if results[0].Error == nil {
			t.Error("expected error when clone fails with CloneOnly=true")
		}
	})

	// CloneOnly=false: clone fails → fallback to InstallCmd
	t.Run("clone fail fallback to install", func(t *testing.T) {
		modules := []Module{
			{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "echo fallback"},
		}
		results := mgr.Install(newWT, modules)
		if len(results) != 1 {
			t.Fatalf(fmtExpectedOneResult, len(results))
		}
		if results[0].Cloned {
			t.Error("expected Cloned=false on clone failure")
		}
	})
}
