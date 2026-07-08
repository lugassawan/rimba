package config_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/resolver"
)

const testProjPrefix = "PROJ-"

// --- PrefixSet tests ---

func TestConfigPrefixSetNilResolver(t *testing.T) {
	cfg := &config.Config{}

	got := cfg.PrefixSet()
	want := resolver.DefaultPrefixSet()

	if !reflect.DeepEqual(got.Strip(), want.Strip()) {
		t.Errorf("PrefixSet().Strip() = %v, want %v", got.Strip(), want.Strip())
	}
	if got.HasCustom() {
		t.Error("PrefixSet().HasCustom() = true, want false for nil Resolver")
	}
}

func TestConfigPrefixSetEmptyResolver(t *testing.T) {
	cfg := &config.Config{Resolver: &config.ResolverConfig{}}

	got := cfg.PrefixSet()
	want := resolver.DefaultPrefixSet()

	if !reflect.DeepEqual(got.Strip(), want.Strip()) {
		t.Errorf("PrefixSet().Strip() = %v, want %v", got.Strip(), want.Strip())
	}
}

func TestConfigPrefixSetCustomEntry(t *testing.T) {
	cfg := &config.Config{
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{
				{Prefix: testProjPrefix, Aliases: []string{"proj"}},
			},
		},
	}

	got := cfg.PrefixSet()

	if !got.HasCustom() {
		t.Error("PrefixSet().HasCustom() = false, want true for custom entry")
	}
	prefix, alias, ok := got.TokenToPrefix("proj")
	if !ok || prefix != testProjPrefix || !alias {
		t.Errorf("TokenToPrefix(%q) = (%q, %v, %v), want (%q, true, true)", "proj", prefix, alias, ok, testProjPrefix)
	}
}

// --- PrefixSetFromContext tests ---

func TestPrefixSetFromContextEmpty(t *testing.T) {
	got := config.PrefixSetFromContext(context.Background())
	if got == nil {
		t.Fatal("PrefixSetFromContext(empty ctx) = nil, want non-nil default set")
	}
	want := resolver.DefaultPrefixSet()
	if !reflect.DeepEqual(got.Strip(), want.Strip()) {
		t.Errorf("PrefixSetFromContext(empty ctx).Strip() = %v, want %v", got.Strip(), want.Strip())
	}
}

func TestPrefixSetFromContextWithCustomConfig(t *testing.T) {
	cfg := &config.Config{
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{
				{Prefix: testProjPrefix, Aliases: []string{"proj"}},
			},
		},
	}
	ctx := config.WithConfig(context.Background(), cfg)

	got := config.PrefixSetFromContext(ctx)
	if got == nil {
		t.Fatal("PrefixSetFromContext(ctx) = nil, want non-nil custom set")
	}
	if !got.HasCustom() {
		t.Error("PrefixSetFromContext(ctx).HasCustom() = false, want true")
	}
	if prefix, _, ok := got.TokenToPrefix("proj"); !ok || prefix != testProjPrefix {
		t.Errorf("TokenToPrefix(%q) = (%q, _, %v), want (%q, _, true)", "proj", prefix, ok, testProjPrefix)
	}
}

// --- validateResolver (via Config.Validate) tests ---

func TestConfigValidateResolver(t *testing.T) {
	tests := []struct {
		name       string
		resolver   *config.ResolverConfig
		wantErr    bool
		wantSubstr []string
	}{
		{
			name:     "nil resolver is valid",
			resolver: nil,
			wantErr:  false,
		},
		{
			name: "valid single entry",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix, Aliases: []string{"proj"}},
				},
			},
			wantErr: false,
		},
		{
			name: "empty prefix rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{{Prefix: ""}},
			},
			wantErr:    true,
			wantSubstr: []string{"resolver.prefix", "prefix is empty"},
		},
		{
			name: "unsafe prefix with space rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{{Prefix: "bad prefix"}},
			},
			wantErr:    true,
			wantSubstr: []string{"resolver.prefix"},
		},
		{
			name: "unsafe prefix with dotdot rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{{Prefix: "../evil"}},
			},
			wantErr:    true,
			wantSubstr: []string{"resolver.prefix"},
		},
		{
			name: "empty alias rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{{Prefix: testProjPrefix, Aliases: []string{""}}},
			},
			wantErr:    true,
			wantSubstr: []string{"resolver.prefix", "alias is empty"},
		},
		{
			name: "alias with slash rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{{Prefix: testProjPrefix, Aliases: []string{"bad/alias"}}},
			},
			wantErr:    true,
			wantSubstr: []string{"resolver.prefix", "bad/alias"},
		},
		{
			name: "duplicate prefix across entries rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix},
					{Prefix: testProjPrefix},
				},
			},
			wantErr:    true,
			wantSubstr: []string{"duplicate prefix", testProjPrefix},
		},
		{
			name: "duplicate alias across entries rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix, Aliases: []string{"proj"}},
					{Prefix: "TASK-", Aliases: []string{"proj"}},
				},
			},
			wantErr:    true,
			wantSubstr: []string{"duplicate alias", "proj"},
		},
		{
			name: "duplicate alias within one entry rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix, Aliases: []string{"proj", "proj"}},
				},
			},
			wantErr:    true,
			wantSubstr: []string{"duplicate alias", "proj"},
		},
		{
			name: "alias shadowing built-in type name on unrelated prefix rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix, Aliases: []string{"bugfix"}},
				},
			},
			wantErr:    true,
			wantSubstr: []string{"shadows built-in prefix", "bugfix/"},
		},
		{
			name: "alias shadowing built-in alias fix on unrelated prefix rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix, Aliases: []string{"fix"}},
				},
			},
			wantErr:    true,
			wantSubstr: []string{"shadows built-in prefix", "bugfix/"},
		},
		{
			name: "entry redeclaring its own built-in prefix with fix alias is ok",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: "bugfix/", Aliases: []string{"fix"}},
				},
			},
			wantErr: false,
		},
		{
			name: "alias shadowing another custom entry's own canonical token rejected",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix},
					{Prefix: "TASK-", Aliases: []string{testProjPrefix}},
				},
			},
			wantErr:    true,
			wantSubstr: []string{"shadows prefix", testProjPrefix},
		},
		{
			name: "entry declaring an alias equal to its own canonical token is ok",
			resolver: &config.ResolverConfig{
				Prefix: []config.PrefixEntry{
					{Prefix: testProjPrefix, Aliases: []string{testProjPrefix, "proj"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				WorktreeDir: "../wt",
				Resolver:    tt.resolver,
			}
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Validate() returned nil, want error containing %v", tt.wantSubstr)
				}
				msg := err.Error()
				for _, sub := range tt.wantSubstr {
					if !strings.Contains(msg, sub) {
						t.Errorf("Validate() error = %q, want substring %q", msg, sub)
					}
				}
			} else if err != nil {
				t.Errorf("Validate() returned unexpected error: %v", err)
			}
		})
	}
}

// --- Merge tests ---

func TestMergeResolverLocalNilPreservesTeam(t *testing.T) {
	team := &config.Config{
		WorktreeDir: "../wt",
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: testProjPrefix}},
		},
	}
	local := &config.Config{}

	merged := config.Merge(team, local)
	if merged.Resolver == nil || len(merged.Resolver.Prefix) != 1 || merged.Resolver.Prefix[0].Prefix != testProjPrefix {
		t.Errorf("Resolver = %+v, want team's Resolver preserved", merged.Resolver)
	}
}

func TestMergeResolverLocalReplacesTeam(t *testing.T) {
	team := &config.Config{
		WorktreeDir: "../wt",
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: testProjPrefix}},
		},
	}
	local := &config.Config{
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{{Prefix: "TASK-"}},
		},
	}

	merged := config.Merge(team, local)
	if merged.Resolver == nil || len(merged.Resolver.Prefix) != 1 || merged.Resolver.Prefix[0].Prefix != "TASK-" {
		t.Errorf("Resolver = %+v, want local's Resolver to replace team's wholesale", merged.Resolver)
	}
}

// --- TOML round-trip test ---

func TestResolverConfigTOMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.toml")

	original := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: "main",
		CopyFiles:     []string{".env"},
		Resolver: &config.ResolverConfig{
			Prefix: []config.PrefixEntry{
				{Prefix: testProjPrefix, Aliases: []string{"proj", "project"}},
				{Prefix: "bugfix/", Aliases: []string{"fix"}},
			},
		},
	}

	if err := config.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Resolver == nil {
		t.Fatal("loaded.Resolver is nil, want populated ResolverConfig")
	}
	if !reflect.DeepEqual(loaded.Resolver, original.Resolver) {
		t.Errorf("loaded.Resolver = %+v, want %+v", loaded.Resolver, original.Resolver)
	}

	rawBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading raw TOML: %v", err)
	}
	raw := string(rawBytes)
	if !strings.Contains(raw, "[[resolver.prefix]]") {
		t.Errorf("raw TOML = %q, want to contain %q", raw, "[[resolver.prefix]]")
	}
	if strings.Contains(raw, "prefixes =") || strings.Contains(raw, "[[resolver.prefixes]]") {
		t.Errorf("raw TOML = %q, must use singular 'prefix' key, not 'prefixes'", raw)
	}
}
