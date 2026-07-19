package deps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/observability"
)

const (
	porcelainWorktree  = "worktree "
	fmtExpectedResults = "expected 1 result(s), got %d"

	fmtExpectedNoError   = "expected no error, got %v"
	fmtExpectedOneResult = "expected 1 result, got %d"
	testDirCustomDeps    = "custom/deps"
)

// progressCall records a single invocation of a progress callback.
type progressCall struct {
	message string
}

// mockRunner implements git.Runner for testing.
type mockRunner struct {
	worktreeOutput string
}

var errGitFailed = errors.New("git worktree list failed")

func (m *mockRunner) Run(_ context.Context, args ...string) (string, error) {
	return m.worktreeOutput, nil
}

func (m *mockRunner) RunInDir(_ context.Context, dir string, args ...string) (string, error) {
	return m.worktreeOutput, nil
}

// withCowEligible forces cowEligible's result for the duration of the test,
// so clone-vs-install routing doesn't depend on the test host's real
// filesystem CoW support. Restored via t.Cleanup.
func withCowEligible(t *testing.T, eligible bool) {
	t.Helper()
	orig := cowEligible
	cowEligible = func(context.Context, string, string) bool { return eligible }
	t.Cleanup(func() { cowEligible = orig })
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

// TestManagerInstallClone covers a non-recursive install-capable module —
// the one shape where cowEligible still gates a real clone attempt (see
// TestManagerInstallRecursiveAlwaysInstalls for why Recursive modules never
// reach this path at all).
func TestManagerInstallClone(t *testing.T) {
	withCowEligible(t, true)

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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Cloned {
		t.Error("expected Cloned=true")
	}
	if !r.Reflink {
		t.Error("expected Reflink=true when cowEligible confirms a true reflink")
	}
	if r.Source != existingWT {
		t.Errorf("expected source %s, got %s", existingWT, r.Source)
	}
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}
	if !r.Ran {
		t.Error("expected Ran=true")
	}

	assertFileContent(t, filepath.Join(newWT, DirNodeModules, "package.json"), "{}")
}

// TestManagerInstallRecursiveAlwaysInstalls verifies the fix for a real
// production monorepo where cowEligible reported a genuine reflink clone
// (not a byte-copy) yet still took 100+s: a Recursive module (pnpm/yarn/npm
// node_modules) walks and clones every nested node_modules dir in a
// monorepo, so its true cost scales with workspace count regardless of true
// CoW support. Even with cowEligible forced true (so a byte-copy-vs-reflink
// bug wouldn't be masked), a Recursive install-capable module must still
// install rather than clone — the sibling's content must never appear.
func TestManagerInstallRecursiveAlwaysInstalls(t *testing.T) {
	withCowEligible(t, true)

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
			InstallCmd: "echo ok",
			Recursive:  true,
		},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false: Recursive install-capable modules always install")
	}
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}
	if _, err := os.Stat(filepath.Join(newWT, DirNodeModules, "package.json")); !os.IsNotExist(err) {
		t.Error("expected the sibling's node_modules to never be cloned")
	}
}

// TestManagerInstallIneligibleCloneFallsThroughToInstall verifies Stage 1's
// headline behavior: an install-capable module with a hash match and an
// existing source dir still runs its InstallCmd instead of cloning when
// cowEligible reports the destination filesystem can't honor a true
// reflink/clonefile — preventing a byte-copy in disguise as a "clone".
func TestManagerInstallIneligibleCloneFallsThroughToInstall(t *testing.T) {
	withCowEligible(t, false)

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
			InstallCmd: "echo ok",
		},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false: ineligible clone must fall through to InstallCmd")
	}
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}

	// The sibling's node_modules must NOT have been byte-copied over.
	if _, err := os.Stat(filepath.Join(newWT, DirNodeModules, "package.json")); !os.IsNotExist(err) {
		t.Error("expected no byte-copy of the sibling's node_modules")
	}
}

// TestManagerInstallCloneOnlyIgnoresIneligibility verifies the plan's
// documented exception: CloneOnly modules (Go vendor, Gradle) have no
// install fallback, so they keep cloning even when cowEligible says the
// filesystem can't honor a true reflink.
func TestManagerInstallCloneOnlyIgnoresIneligibility(t *testing.T) {
	withCowEligible(t, false)

	existingWT := t.TempDir()
	newWT := t.TempDir()

	writeFile(t, existingWT, LockfileGo, "go-sum-content")
	writeFile(t, newWT, LockfileGo, "go-sum-content")

	if err := os.MkdirAll(filepath.Join(existingWT, DirVendor), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existingWT, DirVendor), "module.go", "package foo")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirVendor, Lockfile: LockfileGo, InstallCmd: "go mod vendor", CloneOnly: true},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Cloned {
		t.Error("expected Cloned=true: CloneOnly modules clone regardless of cowEligible")
	}
	if r.Reflink {
		t.Error("expected Reflink=false when cowEligible reports the fs can't honor a true reflink")
	}
	assertFileContent(t, filepath.Join(newWT, DirVendor, "module.go"), "package foo")
}

// TestManagerInstallNoInstallCmdIgnoresIneligibility verifies a module with
// no InstallCmd at all (nothing to fall back to) always attempts the clone,
// matching pre-Stage-1 behavior.
func TestManagerInstallNoInstallCmdIgnoresIneligibility(t *testing.T) {
	withCowEligible(t, false)

	existingWT := t.TempDir()
	newWT := t.TempDir()

	writeFile(t, existingWT, LockfilePnpm, "lockfile-content")
	writeFile(t, newWT, LockfilePnpm, "lockfile-content")

	if err := os.MkdirAll(filepath.Join(existingWT, DirNodeModules), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(existingWT, DirNodeModules), "package.json", "{}")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm}, // no InstallCmd
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Cloned {
		t.Error("expected Cloned=true: no InstallCmd means there's no fallback to prefer")
	}
}

func TestModuleSpanDetail(t *testing.T) {
	tests := []struct {
		name   string
		result InstallResult
		want   string
	}{
		{name: "installed", result: InstallResult{Cloned: false}, want: observability.DetailInstalled},
		{name: "cloned-reflink", result: InstallResult{Cloned: true, Reflink: true}, want: observability.DetailClonedReflink},
		{name: "cloned-copy", result: InstallResult{Cloned: true, Reflink: false}, want: observability.DetailClonedCopy},
		{name: "deferred", result: InstallResult{Deferred: true}, want: observability.DetailDeferred},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := moduleSpanDetail(tt.result); got != tt.want {
				t.Errorf("moduleSpanDetail(%+v) = %q, want %q", tt.result, got, tt.want)
			}
		})
	}
}

func TestManagerSkipDeferredSkipsLazyModule(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "lockfile-content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner, SkipDeferred: true}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "echo ok", Recursive: true, Eager: false},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if !r.Deferred {
		t.Error("expected Deferred=true when SkipDeferred is set and the module isn't Eager")
	}
	if r.Cloned || r.Error != nil {
		t.Errorf("expected no clone/install attempt for a deferred module, got Cloned=%v Error=%v", r.Cloned, r.Error)
	}
	if _, err := os.Stat(filepath.Join(newWT, DirNodeModules)); !os.IsNotExist(err) {
		t.Error("expected node_modules to not exist: deferred modules are never materialized")
	}
}

func TestManagerSkipDeferredStillInstallsEagerModule(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfileGo, "go-sum-content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner, SkipDeferred: true}
	modules := []Module{
		{Dir: DirVendor, Lockfile: LockfileGo, InstallCmd: "echo ok", Eager: true},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}
	if results[0].Deferred {
		t.Error("expected Deferred=false for an Eager module even with SkipDeferred set")
	}
	if results[0].Error != nil {
		t.Errorf(fmtExpectedNoError, results[0].Error)
	}
}

func TestManagerWithoutSkipDeferredIgnoresEager(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "lockfile-content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner} // SkipDeferred defaults to false
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "echo ok", Recursive: true, Eager: false},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}
	if results[0].Deferred {
		t.Error("expected Deferred=false: SkipDeferred is false, so Eager is ignored (matches `rimba deps install`'s explicit-ask semantics)")
	}
	if results[0].Error != nil {
		t.Errorf(fmtExpectedNoError, results[0].Error)
	}
}

func TestManagerInstallPostCloneError(t *testing.T) {
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

	postCloneErr := errors.New("post-clone failure")
	modules := []Module{
		{
			Dir:      DirNodeModules,
			Lockfile: LockfilePnpm,
			PostClone: func(srcWT, dstWT string, mod Module) error {
				return postCloneErr
			},
		},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}

	r := results[0]
	if r.Error == nil {
		t.Error("expected error from PostClone")
	}
	if !errors.Is(r.Error, postCloneErr) {
		t.Errorf("expected postCloneErr in error chain, got %v", r.Error)
	}
	if r.Cloned {
		t.Error("expected Cloned=false on PostClone error")
	}
	// Verify dst dir was removed on PostClone error
	if _, err := os.Stat(filepath.Join(newWT, DirNodeModules)); !os.IsNotExist(err) {
		t.Error("expected cloned dir to be removed on PostClone error")
	}
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)

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

	results := mgr.InstallPreferSource(context.Background(), newWT, sourceWT, modules, nil, nil)
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

	modules, err := ResolveModules(dir, "", true, nil, []string{wt1})
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

	modules, err := ResolveModules(dir, "", false, configModules, nil)
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

	modules, err := ResolveModules(dir, "", true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if modules != nil {
		t.Errorf("expected nil modules, got %v", modules)
	}
}

func TestResolveModulesNoAutoDetectNoConfig(t *testing.T) {
	dir := t.TempDir()

	modules, err := ResolveModules(dir, "", false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if modules != nil {
		t.Errorf("expected nil modules, got %v", modules)
	}
}

func TestResolveModulesNoAutoDetectSkipsPatchOnlyEntry(t *testing.T) {
	dir := t.TempDir()

	// A patch-only entry (no Lockfile/Install) has nothing to patch when
	// auto-detection is off — it must be skipped, not turned into a broken
	// module with an empty Lockfile (which would crash HashModules).
	configModules := []config.ModuleConfig{{Dir: testDirCustomDeps}}

	modules, err := ResolveModules(dir, "", false, configModules, nil)
	if err != nil {
		t.Fatal(err)
	}
	if modules != nil {
		t.Errorf("expected nil modules, got %v", modules)
	}

	if _, err := HashModules(dir, modules); err != nil {
		t.Errorf("HashModules on skipped entry: %v", err)
	}
}

func TestResolveModulesFilterCloneOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfileGo, "go.sum content")

	// No existing worktrees have vendor/ → clone-only should be filtered out
	modules, err := ResolveModules(dir, "", true, nil, nil)
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error for unreadable lockfile")
	}
	if results[0].Ran {
		t.Error("expected Ran=false for an undispatched batch")
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
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

	results := mgr.InstallPreferSource(context.Background(), newWT, sourceWT, modules, nil, nil)
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	// Hash mismatch means no clone, falls through to InstallCmd
	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false (hash mismatch)")
	}
}

// TestManagerInstallRecordsModuleSpanAndExecSubprocess verifies installModule
// records a "deps:<dir>" module span (with the correct cloned/installed
// detail) and runInstall records a CategoryExec subprocess, both via the
// Recorder attached to ctx.
func TestManagerInstallRecordsModuleSpanAndExecSubprocess(t *testing.T) {
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

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "task", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	results := mgr.Install(ctx, newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error != nil {
		t.Fatalf(fmtExpectedNoError, results[0].Error)
	}

	if len(sink.metrics) != 1 {
		t.Fatalf("len(sink.metrics) = %d, want 1", len(sink.metrics))
	}
	span, ok := sink.metrics[0].(observability.SpanRecord)
	if !ok {
		t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
	}
	if span.Name != "deps:"+DirNodeModules {
		t.Errorf("span.Name = %q, want %q", span.Name, "deps:"+DirNodeModules)
	}
	if span.Detail != "installed" {
		t.Errorf("span.Detail = %q, want %q (no clone source, falls through to InstallCmd)", span.Detail, "installed")
	}

	if len(sink.logs) != 1 {
		t.Fatalf("len(sink.logs) = %d, want 1", len(sink.logs))
	}
	subRec, ok := sink.logs[0].(observability.SubprocessRecord)
	if !ok {
		t.Fatalf("sink.logs[0] = %T, want SubprocessRecord", sink.logs[0])
	}
	if subRec.Category != observability.CategoryExec {
		t.Errorf("Category = %q, want %q", subRec.Category, observability.CategoryExec)
	}
	if subRec.Outcome != observability.OutcomeSuccess {
		t.Errorf("Outcome = %q, want %q", subRec.Outcome, observability.OutcomeSuccess)
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
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

	results := mgr.InstallPreferSource(context.Background(), newWT, sourceWT, modules, nil, nil)
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

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
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

func (e *errorRunner) Run(_ context.Context, _ ...string) (string, error) { return "", e.err }
func (e *errorRunner) RunInDir(_ context.Context, _ string, _ ...string) (string, error) {
	return "", e.err
}

func TestInstallListWorktreesError(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "content")

	mgr := &Manager{Runner: &errorRunner{err: errGitFailed}}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error from ListWorktrees failure")
	}
	if !errors.Is(results[0].Error, errGitFailed) {
		t.Errorf("error = %v, want %v", results[0].Error, errGitFailed)
	}
	if results[0].Ran {
		t.Error("expected Ran=false for an undispatched batch")
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

	results := mgr.InstallPreferSource(context.Background(), newWT, sourceWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error from ListWorktrees failure")
	}
	if !errors.Is(results[0].Error, errGitFailed) {
		t.Errorf("error = %v, want %v", results[0].Error, errGitFailed)
	}
	if results[0].Ran {
		t.Error("expected Ran=false for an undispatched batch")
	}
}

func TestManagerInstallCloneFailFallback(t *testing.T) {
	// Force eligible=true so both subtests exercise a genuine CloneModule
	// failure (the real `cp` against a read-only newWT) rather than being
	// intercepted earlier by the ineligibility gate — which, on a read-only
	// dst, would otherwise short-circuit here too (probeCowCapable can't
	// create its temp file there either).
	withCowEligible(t, true)

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
		results := mgr.Install(context.Background(), newWT, modules, nil, nil)
		if len(results) != 1 {
			t.Fatalf(fmtExpectedOneResult, len(results))
		}
		if results[0].Error == nil {
			t.Error("expected error when clone fails with CloneOnly=true")
		}
	})

	// CloneOnly=false, genuinely eligible clone whose real CloneModule call
	// still fails (read-only dst) → cowCopy propagates the raw error (no
	// byte-copy retry, per Stage 1) and cloneAndPost falls back to InstallCmd.
	t.Run("clone fail fallback to install", func(t *testing.T) {
		modules := []Module{
			{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "echo fallback"},
		}
		results := mgr.Install(context.Background(), newWT, modules, nil, nil)
		if len(results) != 1 {
			t.Fatalf(fmtExpectedOneResult, len(results))
		}
		if results[0].Cloned {
			t.Error("expected Cloned=false on clone failure")
		}
	})
}

func TestManagerInstallWithWorkDir(t *testing.T) {
	existingWT := t.TempDir()
	newWT := t.TempDir()

	// Create a subdirectory for WorkDir
	subDir := filepath.Join(newWT, "frontend")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	lockContent := "same-lock"
	writeFile(t, existingWT, LockfilePnpm, lockContent)
	writeFile(t, newWT, LockfilePnpm, lockContent)

	// No matching module dir in existingWT → falls through to InstallCmd with WorkDir
	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}

	modules := []Module{
		{
			Dir:        DirNodeModules,
			Lockfile:   LockfilePnpm,
			InstallCmd: "echo ok",
			WorkDir:    "frontend",
		},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	if results[0].Error != nil {
		t.Errorf(fmtExpectedNoError, results[0].Error)
	}
}

func TestManagerInstallNoInstallCmd(t *testing.T) {
	// Module with a lockfile (hash != ""), no matching source has the
	// module dir, CloneOnly=false, InstallCmd="" → falls through to
	// the final return at installModule line 136.
	existingWT := t.TempDir()
	newWT := t.TempDir()

	lockContent := "same-content"
	writeFile(t, existingWT, LockfilePnpm, lockContent)
	writeFile(t, newWT, LockfilePnpm, lockContent)

	// existingWT does NOT have node_modules → no match
	runner := &mockRunner{worktreeOutput: mockWorktreeList(existingWT, newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm}, // no InstallCmd
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedOneResult, len(results))
	}
	r := results[0]
	if r.Cloned {
		t.Error("expected Cloned=false")
	}
	if r.Error != nil {
		t.Errorf(fmtExpectedNoError, r.Error)
	}
}

func TestRunInstallError(t *testing.T) {
	dir := t.TempDir()
	mod := Module{
		Dir:        DirNodeModules,
		InstallCmd: "/nonexistent-command-xyz",
	}

	err := runInstall(context.Background(), dir, mod)
	if err == nil {
		t.Fatal("expected error from runInstall with invalid command")
	}
	if !strings.Contains(err.Error(), "install") {
		t.Errorf("error = %q, want to contain 'install'", err.Error())
	}
}

func TestInstallProgressCallback(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "lock-content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	// Force sequential so progress messages appear in a predictable order.
	mgr := &Manager{Runner: runner, Concurrency: 1}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "echo ok"},
		{Dir: DirVendor, Lockfile: LockfileGo},
	}

	var mu sync.Mutex
	var calls []progressCall
	onProgress := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, progressCall{msg})
	}

	mgr.Install(context.Background(), newWT, modules, nil, onProgress)

	if len(calls) != 2 {
		t.Fatalf("expected 2 progress calls, got %d", len(calls))
	}
	if want := "1/2 complete"; calls[0].message != want {
		t.Errorf("calls[0] = %q, want %q", calls[0].message, want)
	}
	if want := "2/2 complete"; calls[1].message != want {
		t.Errorf("calls[1] = %q, want %q", calls[1].message, want)
	}
}

func TestInstallPreservesOrderWhenParallel(t *testing.T) {
	// Use distinct InstallCmds with different sleep durations so completion
	// order differs from input order. Results must still match input order.
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "lock")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner, Concurrency: 4}

	// Create unique WorkDir subdirectories so each module runs in its own dir.
	modules := make([]Module, 5)
	for i := range modules {
		sub := fmt.Sprintf("m%d", i)
		if err := os.MkdirAll(filepath.Join(newWT, sub), 0755); err != nil {
			t.Fatal(err)
		}
		// Earlier modules sleep longer so they complete last.
		sleep := strconv.Itoa(5 - i) // module 0 sleeps 5cs, module 4 sleeps 1cs
		modules[i] = Module{
			Dir:        sub,
			Lockfile:   LockfilePnpm,
			InstallCmd: fmt.Sprintf("sleep 0.0%s && echo %s", sleep, sub),
			WorkDir:    sub,
		}
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)

	if len(results) != len(modules) {
		t.Fatalf("expected %d results, got %d", len(modules), len(results))
	}
	for i, r := range results {
		if r.Module.Dir != modules[i].Dir {
			t.Errorf("results[%d].Module.Dir = %q, want %q (order not preserved)",
				i, r.Module.Dir, modules[i].Dir)
		}
		if r.Error != nil {
			t.Errorf("results[%d] unexpected error: %v", i, r.Error)
		}
	}
}

func TestInstallParallelErrorIsolation(t *testing.T) {
	// One failing module must not prevent others from running.
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "lock")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner, Concurrency: 3}

	for _, sub := range []string{"a", "b", "c"} {
		if err := os.MkdirAll(filepath.Join(newWT, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}

	modules := []Module{
		{Dir: "a", Lockfile: LockfilePnpm, InstallCmd: "echo ok-a", WorkDir: "a"},
		{Dir: "b", Lockfile: LockfilePnpm, InstallCmd: "/nonexistent-xyz-command", WorkDir: "b"},
		{Dir: "c", Lockfile: LockfilePnpm, InstallCmd: "echo ok-c", WorkDir: "c"},
	}

	results := mgr.Install(context.Background(), newWT, modules, nil, nil)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("module a: unexpected error %v", results[0].Error)
	}
	if results[1].Error == nil {
		t.Error("module b: expected error for nonexistent command, got nil")
	}
	if results[2].Error != nil {
		t.Errorf("module c: unexpected error %v", results[2].Error)
	}
}

func TestManagerResolveConcurrencyExplicit(t *testing.T) {
	m := &Manager{Concurrency: 7}
	if got := m.resolveConcurrency(); got != 7 {
		t.Errorf("resolveConcurrency() = %d, want 7", got)
	}
}

func TestManagerResolveConcurrencyAuto(t *testing.T) {
	m := &Manager{} // Concurrency == 0
	got := m.resolveConcurrency()
	if got < 1 || got > defaultDepsConcurrencyCap {
		t.Errorf("resolveConcurrency() = %d, want in [1, %d]", got, defaultDepsConcurrencyCap)
	}
}

func TestInstallOutputCapture(t *testing.T) {
	dir := t.TempDir()
	mod := Module{
		Dir:        DirNodeModules,
		InstallCmd: "echo captured-output && exit 1",
	}

	err := runInstall(context.Background(), dir, mod)
	if err == nil {
		t.Fatal("expected error from runInstall")
	}
	if !strings.Contains(err.Error(), "captured-output") {
		t.Errorf("error should contain captured output, got %q", err.Error())
	}
}

func TestRunInstallOutputTailCapped(t *testing.T) {
	// Regression test for the bounded tail buffer: a failing command that
	// writes more than outputTailCapBytes of output must produce an error
	// message containing only the tail — the earliest-written marker should
	// be dropped, the latest-written marker must survive.
	dir := t.TempDir()
	fillerSize := outputTailCapBytes + 4096
	// The markers are split across separate printf calls in the command
	// source so the contiguous marker text only ever appears in the
	// captured *output*, never in InstallCmd itself — errhint.WithFix
	// embeds the raw InstallCmd in its "to fix" hint, which would otherwise
	// make this assertion pass regardless of whether output capping works.
	mod := Module{
		Dir: DirNodeModules,
		InstallCmd: fmt.Sprintf(
			"printf 'EARLY'; printf 'MARKER'; yes x | head -c %d; printf 'LATE'; printf 'MARKERTAIL'; exit 1",
			fillerSize),
	}

	err := runInstall(context.Background(), dir, mod)
	if err == nil {
		t.Fatal("expected error from runInstall")
	}
	if strings.Contains(err.Error(), "EARLYMARKER") {
		t.Error("error contains the earliest-written output, want it dropped by the tail cap")
	}
	if !strings.Contains(err.Error(), "LATEMARKERTAIL") {
		t.Error("error should contain the latest-written output")
	}
}

func TestInstallModuleRecordsModuleSpanCloneHit(t *testing.T) {
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
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "pnpm install --frozen-lockfile"},
	}

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "task", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	results := mgr.Install(ctx, newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}
	if !results[0].Cloned {
		t.Fatal("expected Cloned=true")
	}

	if len(sink.metrics) != 1 {
		t.Fatalf("len(sink.metrics) = %d, want 1", len(sink.metrics))
	}
	span, ok := sink.metrics[0].(observability.SpanRecord)
	if !ok {
		t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
	}
	if span.Name != "deps:"+DirNodeModules {
		t.Errorf("span.Name = %q, want %q", span.Name, "deps:"+DirNodeModules)
	}
	if span.Detail != "cloned" {
		t.Errorf("span.Detail = %q, want %q", span.Detail, "cloned")
	}
}

func TestInstallModuleRecordsModuleSpanOnInstallError(t *testing.T) {
	newWT := t.TempDir()
	writeFile(t, newWT, LockfilePnpm, "lockfile-content")

	runner := &mockRunner{worktreeOutput: mockWorktreeList(newWT)}
	mgr := &Manager{Runner: runner}
	modules := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "/nonexistent-command-xyz"},
	}

	sink := &fakeSink{}
	rec := observability.Maybe(true, sink, "add", "task", "", "v1")
	ctx := observability.WithRecorder(context.Background(), rec)

	results := mgr.Install(ctx, newWT, modules, nil, nil)
	if len(results) != 1 {
		t.Fatalf(fmtExpectedResults, len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected error from invalid install command")
	}

	if len(sink.metrics) != 1 {
		t.Fatalf("len(sink.metrics) = %d, want 1", len(sink.metrics))
	}
	span, ok := sink.metrics[0].(observability.SpanRecord)
	if !ok {
		t.Fatalf("sink.metrics[0] = %T, want SpanRecord", sink.metrics[0])
	}
	if span.Name != "deps:"+DirNodeModules {
		t.Errorf("span.Name = %q, want %q", span.Name, "deps:"+DirNodeModules)
	}
	if span.Detail != "installed" {
		t.Errorf("span.Detail = %q, want %q (a span is still recorded when install fails)", span.Detail, "installed")
	}
}

func TestResolveModulesConfigWithWorkDir(t *testing.T) {
	dir := t.TempDir()

	configModules := []config.ModuleConfig{
		{Dir: "frontend/node_modules", Lockfile: "frontend/pnpm-lock.yaml", Install: "pnpm install", WorkDir: "frontend"},
	}

	modules, err := ResolveModules(dir, "", false, configModules, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}
	if modules[0].WorkDir != "frontend" {
		t.Errorf("WorkDir = %q, want %q", modules[0].WorkDir, "frontend")
	}
}
