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

	modules, err := DetectModules(dir, "")
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

	modules, err := DetectModules(dir, "")
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

	modules, err := DetectModules(dir, "")
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

func TestDetectModulesRust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfileCargo, "[package]")

	modules, err := DetectModules(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)

	m := modules[0]
	if m.Dir != DirTarget {
		t.Errorf(fmtExpectedGot, DirTarget, m.Dir)
	}
	if m.Lockfile != LockfileCargo {
		t.Errorf(fmtExpectedGot, LockfileCargo, m.Lockfile)
	}
	if !m.CloneOnly {
		t.Error("expected CloneOnly=true for Rust")
	}
	if m.Recursive {
		t.Error("expected Recursive=false for Rust")
	}
	if m.InstallCmd != "" {
		t.Errorf("expected empty InstallCmd, got %s", m.InstallCmd)
	}
}

func TestDetectModulesUv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfileUv, "uv-lock-content")

	modules, err := DetectModules(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)

	m := modules[0]
	if m.Dir != DirVenv {
		t.Errorf(fmtExpectedGot, DirVenv, m.Dir)
	}
	if m.Lockfile != LockfileUv {
		t.Errorf(fmtExpectedGot, LockfileUv, m.Lockfile)
	}
	if !m.CloneOnly {
		t.Error("CloneOnly should be true")
	}
	if m.PostClone == nil {
		t.Error("PostClone should be set (non-nil)")
	}
}

func TestDetectModulesPoetry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfilePoetry, "poetry-lock-content")

	modules, err := DetectModules(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)

	m := modules[0]
	if m.Dir != DirVenv {
		t.Errorf(fmtExpectedGot, DirVenv, m.Dir)
	}
	if m.Lockfile != LockfilePoetry {
		t.Errorf(fmtExpectedGot, LockfilePoetry, m.Lockfile)
	}
	if !m.CloneOnly {
		t.Error("CloneOnly should be true")
	}
	if m.PostClone == nil {
		t.Error("PostClone should be set (non-nil)")
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

	modules, err := DetectModules(dir, "")
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

func TestDetectModulesScopedToService(t *testing.T) {
	dir := t.TempDir()

	// Root: pnpm
	writeFile(t, dir, LockfilePnpm, "lockfile-v6")

	// Two service subdirs with lockfiles
	for _, svc := range []string{"auth-api", "web-app"} {
		if err := os.MkdirAll(filepath.Join(dir, svc), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dir, "auth-api"), LockfileGo, "hash-go")
	writeFile(t, filepath.Join(dir, "web-app"), LockfileNpm, "{}")

	// Scoped to auth-api: root pnpm + auth-api/go.sum only
	modules, err := DetectModules(dir, "auth-api")
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 2)
	if modules[0].Lockfile != LockfilePnpm {
		t.Errorf("expected root module %s, got %s", LockfilePnpm, modules[0].Lockfile)
	}
	if modules[1].Lockfile != filepath.Join("auth-api", LockfileGo) {
		t.Errorf("expected auth-api/go.sum, got %s", modules[1].Lockfile)
	}

	// Full scan: root pnpm + auth-api/go.sum + web-app/npm
	all, err := DetectModules(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	assertModuleCount(t, all, 3)
}

func TestDetectModulesNoLockfiles(t *testing.T) {
	dir := t.TempDir()

	modules, err := DetectModules(dir, "")
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

	modules, err := DetectModules(dir, "")
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

func TestMergeWithConfigPatchesOnlySetFields(t *testing.T) {
	detected := []Module{
		{Dir: DirNodeModules, Lockfile: LockfilePnpm, InstallCmd: "pnpm install --frozen-lockfile", Recursive: true},
	}
	configModules := []config.ModuleConfig{
		{Dir: DirNodeModules, Install: "pnpm install --frozen-lockfile --prefer-offline"},
	}

	merged := MergeWithConfig(detected, configModules)

	assertModuleCount(t, merged, 1)
	m := merged[0]
	if m.InstallCmd != "pnpm install --frozen-lockfile --prefer-offline" {
		t.Errorf("expected patched InstallCmd, got %q", m.InstallCmd)
	}
	if m.Lockfile != LockfilePnpm {
		t.Errorf("expected inherited Lockfile %q, got %q", LockfilePnpm, m.Lockfile)
	}
	if !m.Recursive {
		t.Error("expected inherited Recursive=true (config can't express this field)")
	}
}

func TestMergeWithConfigNewModuleStillNeedsFullDefinition(t *testing.T) {
	configModules := []config.ModuleConfig{
		{Dir: testDirCustomDeps, Lockfile: "custom.lock", Install: "custom-install"},
	}

	merged := MergeWithConfig(nil, configModules)

	assertModuleCount(t, merged, 1)
	if merged[0].Lockfile != "custom.lock" || merged[0].InstallCmd != "custom-install" {
		t.Errorf("expected brand-new module fully defined from config, got %+v", merged[0])
	}
}

// TestMergeWithConfigUnmatchedPatchOnlyEntryIsNoOp verifies a real production
// bug: a patch-only config entry (dir + eager, no lockfile/install) whose Dir
// isn't in `detected` — e.g. because a --service-scoped DetectModules call
// never looked at that subdirectory at all — must be silently skipped, not
// added as a broken "new module" with an empty Lockfile. An empty Lockfile
// makes HashLockfile try to read the worktree directory itself, erroring out
// hashing for the ENTIRE batch (including otherwise-fine modules like a
// correctly detected root module).
func TestMergeWithConfigUnmatchedPatchOnlyEntryIsNoOp(t *testing.T) {
	detected := []Module{
		{Dir: DirNodeModules, Lockfile: LockfileYarn, InstallCmd: "yarn install", Recursive: true},
	}
	configModules := []config.ModuleConfig{
		{Dir: "internal-cli/node_modules", Eager: new(true)},
	}

	merged := MergeWithConfig(detected, configModules)

	assertModuleCount(t, merged, 1)
	if merged[0].Dir != DirNodeModules {
		t.Errorf("expected only the detected root module to survive, got %+v", merged)
	}
}

func TestMergeWithConfigMatchedPatchOnlyEntryStillPatches(t *testing.T) {
	detected := []Module{
		{Dir: "internal-cli/node_modules", Lockfile: "internal-cli/package-lock.json", InstallCmd: "npm ci", WorkDir: "internal-cli"},
	}
	configModules := []config.ModuleConfig{
		{Dir: "internal-cli/node_modules", Eager: new(true)},
	}

	merged := MergeWithConfig(detected, configModules)

	assertModuleCount(t, merged, 1)
	if merged[0].Lockfile != "internal-cli/package-lock.json" || merged[0].InstallCmd != "npm ci" {
		t.Errorf("expected the matched entry to still patch (inheriting lockfile/install), got %+v", merged[0])
	}
}

func TestModuleInstallState(t *testing.T) {
	tests := []struct {
		name      string
		eager     bool
		createDir bool
		want      string
	}{
		{name: "installed", eager: true, createDir: true, want: "installed"},
		{name: "missing", eager: true, createDir: false, want: "missing"},
		{name: "deferred", eager: false, createDir: false, want: "deferred"},
		{name: "installed even if lazy", eager: false, createDir: true, want: "installed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mod := Module{Dir: DirNodeModules, Eager: tt.eager}
			if tt.createDir {
				if err := os.MkdirAll(filepath.Join(dir, DirNodeModules), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if got := mod.InstallState(dir); got != tt.want {
				t.Errorf("InstallState() = %q, want %q", got, tt.want)
			}
		})
	}
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

	modules, err := DetectModules(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(modules) != 0 {
		t.Errorf("expected 0 modules (hidden dirs skipped), got %d", len(modules))
	}
}

func TestDetectSubdirModulesReadDirError(t *testing.T) {
	dir := t.TempDir()

	// Make directory unreadable so ReadDir fails
	if err := os.Chmod(dir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	seenDirs := make(map[string]bool)
	result := detectSubdirModules(dir, "", nil, seenDirs)

	if len(result) != 0 {
		t.Errorf("expected 0 modules on ReadDir error, got %d", len(result))
	}
}

func TestMatchPresetsInSubdirSeenDirSkip(t *testing.T) {
	dir := t.TempDir()

	// Create a subdir with a lockfile
	subdir := "api"
	if err := os.MkdirAll(filepath.Join(dir, subdir), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, subdir), LockfileGo, "hash")

	// Pre-populate seenDirs with the expected dep dir
	seenDirs := map[string]bool{
		filepath.Join(subdir, DirVendor): true,
	}

	result := matchPresetsInSubdir(dir, subdir, nil, seenDirs)

	if len(result) != 0 {
		t.Errorf("expected 0 modules when seenDirs already has entry, got %d", len(result))
	}
}

func TestDetectModulesGradleEachTrigger(t *testing.T) {
	triggers := []struct {
		lockfile string
	}{
		{LockfileGradleSettings},
		{LockfileGradleSettingsKts},
		{LockfileGradle},
		{LockfileGradleKts},
	}

	for _, tc := range triggers {
		t.Run(tc.lockfile, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, tc.lockfile, "# gradle")

			modules, err := DetectModules(dir, "")
			if err != nil {
				t.Fatal(err)
			}

			assertModuleCount(t, modules, 1)
			m := modules[0]

			if m.Dir != DirGradle {
				t.Errorf(fmtExpectedGot, DirGradle, m.Dir)
			}
			if len(m.ExtraDirs) != 1 || m.ExtraDirs[0] != DirGradleBuildOutput {
				t.Errorf("expected ExtraDirs=[%s], got %v", DirGradleBuildOutput, m.ExtraDirs)
			}
			if !m.CloneOnly {
				t.Error("expected CloneOnly=true for Gradle")
			}
			if m.Recursive {
				t.Error("expected Recursive=false for Gradle")
			}
			if m.InstallCmd != "" {
				t.Errorf("expected empty InstallCmd, got %s", m.InstallCmd)
			}
		})
	}
}

func TestDetectModulesGradleSettingsWinsOverBuild(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, LockfileGradleSettings, "# settings")
	writeFile(t, dir, LockfileGradle, "# build")

	modules, err := DetectModules(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)
	if modules[0].Lockfile != LockfileGradleSettings {
		t.Errorf("expected settings.gradle to win, got %s", modules[0].Lockfile)
	}
}

func TestDetectModulesGradleNestedSubdir(t *testing.T) {
	dir := t.TempDir()
	subproject := "subproject"
	if err := os.MkdirAll(filepath.Join(dir, subproject), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, subproject), LockfileGradleKts, "# kts")

	modules, err := DetectModules(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	assertModuleCount(t, modules, 1)
	m := modules[0]

	if m.Dir != filepath.Join(subproject, DirGradle) {
		t.Errorf("expected %s, got %s", filepath.Join(subproject, DirGradle), m.Dir)
	}
	if m.WorkDir != subproject {
		t.Errorf("expected WorkDir=%s, got %s", subproject, m.WorkDir)
	}
	if !m.CloneOnly {
		t.Error("expected CloneOnly=true for nested Gradle")
	}
	wantExtra := filepath.Join(subproject, DirGradleBuildOutput)
	if len(m.ExtraDirs) != 1 || m.ExtraDirs[0] != wantExtra {
		t.Errorf("expected ExtraDirs=[%s], got %v", wantExtra, m.ExtraDirs)
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
