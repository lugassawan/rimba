package deps

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestResolveEagernessRecursiveDefaultsLazy(t *testing.T) {
	modules := []Module{{Dir: DirNodeModules, Recursive: true}}
	got := resolveEagerness(t.TempDir(), "", modules, nil)
	if got[0].Eager {
		t.Error("expected Recursive module to default to lazy (Eager=false)")
	}
}

func TestResolveEagernessNonRecursiveDefaultsEager(t *testing.T) {
	modules := []Module{{Dir: DirVendor, CloneOnly: true}}
	got := resolveEagerness(t.TempDir(), "", modules, nil)
	if !got[0].Eager {
		t.Error("expected non-Recursive module to default to eager")
	}
}

func TestResolveEagernessExplicitConfigOverrideWins(t *testing.T) {
	modules := []Module{{Dir: DirNodeModules, Recursive: true}}
	configModules := []config.ModuleConfig{{Dir: DirNodeModules, Eager: new(true)}}
	got := resolveEagerness(t.TempDir(), "", modules, configModules)
	if !got[0].Eager {
		t.Error("expected explicit config Eager=true to override the Recursive-lazy default")
	}
}

func TestResolveEagernessExplicitConfigOverrideWinsOverServiceScope(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-frontend"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "app-frontend"), "package.json", "{}")

	modules := []Module{{Dir: DirNodeModules, Recursive: true}} // root, no WorkDir
	configModules := []config.ModuleConfig{{Dir: DirNodeModules, Eager: new(false)}}

	got := resolveEagerness(dir, "app-frontend", modules, configModules)
	if got[0].Eager {
		t.Error("expected explicit config Eager=false to win even though service-scope would otherwise force eager")
	}
}

func TestResolveEagernessServiceMatchesOwnLockfileModule(t *testing.T) {
	modules := []Module{
		{Dir: DirNodeModules, Recursive: true}, // root
		{Dir: filepath.Join("standalone-svc-a", DirNodeModules), WorkDir: "standalone-svc-a", Recursive: true},
	}
	got := resolveEagerness(t.TempDir(), "standalone-svc-a", modules, nil)

	if got[0].Eager {
		t.Error("expected root to stay lazy — service matched a different, standalone module")
	}
	if !got[1].Eager {
		t.Error("expected standalone-svc-a's own module to become eager")
	}
}

func TestResolveEagernessServiceIsWorkspaceMemberForcesRootEager(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app-frontend"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "app-frontend"), "package.json", "{}") // no lockfile of its own

	modules := []Module{{Dir: DirNodeModules, Recursive: true}} // root, WorkDir==""
	got := resolveEagerness(dir, "app-frontend", modules, nil)

	if !got[0].Eager {
		t.Error("expected root to become eager: app-frontend has package.json but no own lockfile, so it's hoisted into root")
	}
}

func TestResolveEagernessServiceIsNonJSForcesNothingEager(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "auth-api-svc"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "auth-api-svc"), "go.sum", "hash") // no package.json at all

	modules := []Module{{Dir: DirNodeModules, Recursive: true}} // root
	got := resolveEagerness(dir, "auth-api-svc", modules, nil)

	if got[0].Eager {
		t.Error("expected root to stay lazy: service has no package.json, so no JS module is implied")
	}
}

func TestResolveEagernessNoServiceNoInference(t *testing.T) {
	modules := []Module{{Dir: DirNodeModules, Recursive: true}}
	got := resolveEagerness(t.TempDir(), "", modules, nil)
	if got[0].Eager {
		t.Error("expected Recursive module to stay lazy with no service given")
	}
}

func TestCoveringRecursiveModuleDirLongestPrefixWins(t *testing.T) {
	modules := []Module{
		{Dir: DirNodeModules, Recursive: true, WorkDir: ""},
		{Dir: filepath.Join("shared", "node", DirNodeModules), Recursive: true, WorkDir: filepath.Join("shared", "node")},
	}
	got := coveringRecursiveModuleDir(filepath.Join("shared", "node", "dashboard-server"), modules)
	want := filepath.Join("shared", "node", DirNodeModules)
	if got != want {
		t.Errorf("coveringRecursiveModuleDir() = %q, want %q (longest WorkDir prefix)", got, want)
	}
}
