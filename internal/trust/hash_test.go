package trust_test

import (
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/lugassawan/rimba/testutil"
)

// Ensure we use testutil (coverage threshold enforcement).
var _ = testutil.Ptr[int]

func emptyConfig() *config.Config {
	return &config.Config{}
}

func cfgWithCommands(postCreate, postRename []string, installs ...string) *config.Config {
	cfg := &config.Config{
		PostCreate: postCreate,
		PostRename: postRename,
	}
	if len(installs) > 0 {
		mods := make([]config.ModuleConfig, 0, len(installs))
		for i, inst := range installs {
			mods = append(mods, config.ModuleConfig{
				Dir:     "mod" + string(rune('a'+i)),
				Install: inst,
			})
		}
		cfg.Deps = &config.DepsConfig{Modules: mods}
	}
	return cfg
}

func TestHashEmptyConfig(t *testing.T) {
	h := trust.Hash(emptyConfig())
	if h != "" {
		t.Errorf("Hash(empty) = %q, want empty string", h)
	}
}

func TestHashHasPrefix(t *testing.T) {
	cfg := cfgWithCommands([]string{"echo hi"}, nil)
	h := trust.Hash(cfg)
	if len(h) < 7 || h[:7] != "sha256:" {
		t.Errorf("Hash = %q, want sha256: prefix", h)
	}
}

func TestHashStableAcrossReorder(t *testing.T) {
	a := trust.Hash(cfgWithCommands([]string{"echo a", "echo b"}, nil))
	b := trust.Hash(cfgWithCommands([]string{"echo b", "echo a"}, nil))
	if a != b {
		t.Errorf("Hash should be stable across reorder: %q != %q", a, b)
	}
}

func TestHashChangesWhenCommandChanges(t *testing.T) {
	a := trust.Hash(cfgWithCommands([]string{"make install"}, nil))
	b := trust.Hash(cfgWithCommands([]string{"make setup"}, nil))
	if a == b {
		t.Errorf("Hash should differ when command changes, got same: %q", a)
	}
}

func TestHashChangesWhenCommandAdded(t *testing.T) {
	a := trust.Hash(cfgWithCommands([]string{"echo a"}, nil))
	b := trust.Hash(cfgWithCommands([]string{"echo a", "echo b"}, nil))
	if a == b {
		t.Errorf("Hash should differ when command is added, got same: %q", a)
	}
}

func TestHashNULDelimiterPreventsCollision(t *testing.T) {
	// ["ab","c"] must differ from ["a","bc"] even after sorting
	a := trust.Hash(cfgWithCommands([]string{"ab", "c"}, nil))
	b := trust.Hash(cfgWithCommands([]string{"a", "bc"}, nil))
	if a == b {
		t.Errorf("Hash should differ for [ab,c] vs [a,bc], got same: %q", a)
	}
}

func TestHashAllFieldsAffectHash(t *testing.T) {
	// Hash is content-based (field-blind by design): restructuring config
	// without adding commands does not require re-consent. Adding a command
	// to ANY field does change the hash.
	base := trust.Hash(cfgWithCommands([]string{"make build"}, nil))
	withRename := trust.Hash(cfgWithCommands([]string{"make build"}, []string{"echo rename"}))
	withDeps := trust.Hash(cfgWithCommands([]string{"make build"}, nil, "npm ci"))
	if base == withRename {
		t.Errorf("adding a post_rename command should change hash")
	}
	if base == withDeps {
		t.Errorf("adding a deps install command should change hash")
	}
	if withRename == withDeps {
		t.Errorf("different extra commands should produce different hashes")
	}
}

func TestHashEmptyInstallSkipped(t *testing.T) {
	// A module with empty Install should not affect the hash.
	cfg := &config.Config{
		PostCreate: []string{"echo hi"},
		Deps: &config.DepsConfig{
			Modules: []config.ModuleConfig{
				{Dir: "vendor", Install: ""},
			},
		},
	}
	a := trust.Hash(cfg)
	b := trust.Hash(cfgWithCommands([]string{"echo hi"}, nil))
	if a != b {
		t.Errorf("empty Install should not affect hash: %q != %q", a, b)
	}
}

func TestHasCommands(t *testing.T) {
	if trust.HasCommands(emptyConfig()) {
		t.Error("HasCommands(empty) should be false")
	}
	if !trust.HasCommands(cfgWithCommands([]string{"x"}, nil)) {
		t.Error("HasCommands with PostCreate should be true")
	}
	if !trust.HasCommands(cfgWithCommands(nil, []string{"x"})) {
		t.Error("HasCommands with PostRename should be true")
	}
	if !trust.HasCommands(cfgWithCommands(nil, nil, "x")) {
		t.Error("HasCommands with deps install should be true")
	}
}

func TestCommandsOrderPreserved(t *testing.T) {
	cfg := cfgWithCommands([]string{"first", "second"}, []string{"third"}, "fourth")
	cmds := trust.Commands(cfg)
	want := []string{"first", "second", "third", "fourth"}
	if len(cmds) != len(want) {
		t.Fatalf("Commands() len = %d, want %d: %v", len(cmds), len(want), cmds)
	}
	for i, w := range want {
		if cmds[i] != w {
			t.Errorf("Commands()[%d] = %q, want %q", i, cmds[i], w)
		}
	}
}

func TestCommandsSkipsEmptyInstall(t *testing.T) {
	cfg := &config.Config{
		Deps: &config.DepsConfig{
			Modules: []config.ModuleConfig{
				{Dir: "a", Install: ""},
				{Dir: "b", Install: "npm ci"},
			},
		},
	}
	cmds := trust.Commands(cfg)
	if len(cmds) != 1 || cmds[0] != "npm ci" {
		t.Errorf("Commands() = %v, want [npm ci]", cmds)
	}
}
