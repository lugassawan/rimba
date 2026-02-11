package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	testDirAPI     = "api"
	fmtExpectedGot = "expected %s, got %s"
)

func TestDetectModulesPnpmPriority(t *testing.T) {
	dir := t.TempDir()

	// Create both pnpm and npm lockfiles — pnpm should win
	writeFile(t, dir, LockfilePnpm, "lockfile-v6")
	writeFile(t, dir, LockfileNpm, "{}")

	modules, err := DetectModules(dir)
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)

	m := modules[0]
	if m.Lockfile != LockfilePnpm {
		t.Errorf(fmtExpectedGot, LockfilePnpm, m.Lockfile)
	}
	if m.Dir != DirNodeModules {
		t.Errorf(fmtExpectedGot, DirNodeModules, m.Dir)
	}
	if !m.Recursive {
		t.Error("expected Recursive=true for pnpm")
	}
}

func TestDetectModulesYarnWithExtraDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfileYarn, "# yarn")

	modules, err := DetectModules(dir)
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)

	m := modules[0]
	if m.Lockfile != LockfileYarn {
		t.Errorf(fmtExpectedGot, LockfileYarn, m.Lockfile)
	}
	if len(m.ExtraDirs) != 1 || m.ExtraDirs[0] != DirYarnCache {
		t.Errorf("expected ExtraDirs=[%s], got %v", DirYarnCache, m.ExtraDirs)
	}
}

func TestDetectModulesNestedLockfile(t *testing.T) {
	dir := t.TempDir()

	// Create a nested Go lockfile
	if err := os.MkdirAll(filepath.Join(dir, testDirAPI), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, testDirAPI), LockfileGo, "hash123")

	modules, err := DetectModules(dir)
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)

	m := modules[0]
	if m.Dir != filepath.Join(testDirAPI, DirVendor) {
		t.Errorf("expected api/vendor, got %s", m.Dir)
	}
	if m.Lockfile != filepath.Join(testDirAPI, LockfileGo) {
		t.Errorf("expected api/go.sum, got %s", m.Lockfile)
	}
	if m.WorkDir != testDirAPI {
		t.Errorf("expected WorkDir=%s, got %s", testDirAPI, m.WorkDir)
	}
	if !m.CloneOnly {
		t.Error("expected CloneOnly=true for Go")
	}
}

func TestDetectModulesPolyglot(t *testing.T) {
	dir := t.TempDir()

	// Root: pnpm
	writeFile(t, dir, LockfilePnpm, "lockfile-v6")

	// Subdir: Go
	if err := os.MkdirAll(filepath.Join(dir, testDirAPI), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, testDirAPI), LockfileGo, "hash123")

	modules, err := DetectModules(dir)
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 2)

	if modules[0].Lockfile != LockfilePnpm {
		t.Errorf("expected first module %s, got %s", LockfilePnpm, modules[0].Lockfile)
	}
	if modules[1].Lockfile != filepath.Join(testDirAPI, LockfileGo) {
		t.Errorf("expected second module api/go.sum, got %s", modules[1].Lockfile)
	}
}

func TestDetectModulesNoLockfiles(t *testing.T) {
	dir := t.TempDir()

	modules, err := DetectModules(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) != 0 {
		t.Errorf("expected 0 modules, got %d", len(modules))
	}
}

func TestDetectModulesRootWinsOverSubdir(t *testing.T) {
	dir := t.TempDir()

	// Root lockfile
	writeFile(t, dir, LockfileGo, "root-hash")

	// Subdir lockfile — root gets "vendor", subdir gets "subdir/vendor"
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "subdir"), LockfileGo, "subdir-hash")

	modules, err := DetectModules(dir)
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 2)

	if modules[0].Dir != DirVendor {
		t.Errorf(fmtExpectedGot, DirVendor, modules[0].Dir)
	}
	if modules[1].Dir != filepath.Join("subdir", DirVendor) {
		t.Errorf("expected subdir/vendor, got %s", modules[1].Dir)
	}
}

func TestFilterCloneOnly(t *testing.T) {
	wt1 := t.TempDir()
	wt2 := t.TempDir()

	// Only wt1 has vendor/
	if err := os.MkdirAll(filepath.Join(wt1, DirVendor), 0755); err != nil {
		t.Fatal(err)
	}

	modules := []Module{
		{Dir: DirNodeModules, CloneOnly: false},
		{Dir: DirVendor, CloneOnly: true},
		{Dir: "api/vendor", CloneOnly: true}, // doesn't exist anywhere
	}

	result := FilterCloneOnly(modules, []string{wt1, wt2})

	assertModuleCount(t, result, 2)
	if result[0].Dir != DirNodeModules {
		t.Errorf(fmtExpectedGot, DirNodeModules, result[0].Dir)
	}
	if result[1].Dir != DirVendor {
		t.Errorf(fmtExpectedGot, DirVendor, result[1].Dir)
	}
}

func TestMergeWithConfigOverride(t *testing.T) {
	detected := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "pnpm install --frozen-lockfile"},
	}

	configModules := []config.ModuleConfig{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, Install: "pnpm install"},
	}

	result := MergeWithConfig(detected, configModules)

	assertModuleCount(t, result, 1)
	if result[0].InstallCmd != "pnpm install" {
		t.Errorf("expected overridden install cmd, got %s", result[0].InstallCmd)
	}
}

func TestMergeWithConfigSupplement(t *testing.T) {
	detected := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm},
	}

	configModules := []config.ModuleConfig{
		{Dir: testDirCustomDeps, Lockfile: "custom.lock", Install: "custom install"},
	}

	result := MergeWithConfig(detected, configModules)

	assertModuleCount(t, result, 2)
	if result[1].Dir != testDirCustomDeps {
		t.Errorf("expected custom/deps, got %s", result[1].Dir)
	}
}

func TestMergeWithConfigEmpty(t *testing.T) {
	detected := []Module{
		{Dir: DirNodeModules},
	}

	result := MergeWithConfig(detected, nil)

	assertModuleCount(t, result, 1)
}

func TestPrefixDirsEmpty(t *testing.T) {
	result := prefixDirs("api", nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}

	result = prefixDirs("api", []string{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %v", result)
	}
}

func TestPrefixDirsNonEmpty(t *testing.T) {
	result := prefixDirs("api", []string{DirYarnCache, DirNodeModules})
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0] != filepath.Join("api", DirYarnCache) {
		t.Errorf("result[0] = %q, want %q", result[0], filepath.Join("api", DirYarnCache))
	}
	if result[1] != filepath.Join("api", DirNodeModules) {
		t.Errorf("result[1] = %q, want %q", result[1], filepath.Join("api", DirNodeModules))
	}
}

func TestDetectModulesHiddenDirSkipped(t *testing.T) {
	dir := t.TempDir()

	// Create a hidden directory with a lockfile — should be skipped
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, hiddenDir, LockfilePnpm, "lockfile")

	modules, err := DetectModules(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) != 0 {
		t.Errorf("expected 0 modules (hidden dirs skipped), got %d", len(modules))
	}
}

func assertModuleCount(t *testing.T, modules []Module, expected int) {
	t.Helper()
	if len(modules) != expected {
		t.Fatalf("expected %d module(s), got %d", expected, len(modules))
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
