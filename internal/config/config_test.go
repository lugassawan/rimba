package config_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	testRepoName      = "test-repo"
	testDefaultBranch = "main"
	testDevelopBranch = "develop"
	fatalSave         = "Save: %v"
	fatalLoad         = "Load: %v"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig("myrepo", testDefaultBranch)

	if cfg.WorktreeDir != "../myrepo-worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, "../myrepo-worktrees")
	}
	if cfg.DefaultSource != testDefaultBranch {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, testDefaultBranch)
	}
	expected := []string{".env", ".env.local", ".envrc", ".tool-versions"}
	if !reflect.DeepEqual(cfg.CopyFiles, expected) {
		t.Errorf("CopyFiles = %v, want %v", cfg.CopyFiles, expected)
	}
}

func TestDefaultCopyFiles(t *testing.T) {
	got := config.DefaultCopyFiles()
	expected := []string{".env", ".env.local", ".envrc", ".tool-versions"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("DefaultCopyFiles() = %v, want %v", got, expected)
	}
}

func TestFillDefaults(t *testing.T) {
	t.Run("fills all empty fields", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.FillDefaults("myrepo", testDefaultBranch)

		if cfg.WorktreeDir != "../myrepo-worktrees" {
			t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, "../myrepo-worktrees")
		}
		if cfg.DefaultSource != testDefaultBranch {
			t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, testDefaultBranch)
		}
		expected := config.DefaultCopyFiles()
		if !reflect.DeepEqual(cfg.CopyFiles, expected) {
			t.Errorf("CopyFiles = %v, want %v", cfg.CopyFiles, expected)
		}
	})

	t.Run("preserves explicit values", func(t *testing.T) {
		cfg := &config.Config{
			WorktreeDir:   "../custom-wt",
			DefaultSource: testDevelopBranch,
			CopyFiles:     []string{".env"},
		}
		cfg.FillDefaults("myrepo", testDefaultBranch)

		if cfg.WorktreeDir != "../custom-wt" {
			t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, "../custom-wt")
		}
		if cfg.DefaultSource != testDevelopBranch {
			t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, testDevelopBranch)
		}
		if !reflect.DeepEqual(cfg.CopyFiles, []string{".env"}) {
			t.Errorf("CopyFiles = %v, want [.env]", cfg.CopyFiles)
		}
	})

	t.Run("preserves explicitly empty CopyFiles", func(t *testing.T) {
		cfg := &config.Config{
			CopyFiles: []string{}, // explicitly empty — should NOT be overridden
		}
		cfg.FillDefaults("myrepo", testDefaultBranch)

		if cfg.CopyFiles == nil || len(cfg.CopyFiles) != 0 {
			t.Errorf("CopyFiles = %v, want empty non-nil slice", cfg.CopyFiles)
		}
	})
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	original := config.DefaultConfig(testRepoName, testDefaultBranch)
	if err := config.Save(path, original); err != nil {
		t.Fatalf(fatalSave, err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf(fatalLoad, err)
	}

	if !reflect.DeepEqual(original, loaded) {
		t.Errorf("loaded config differs:\n  got:  %+v\n  want: %+v", loaded, original)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := config.Load(filepath.Join("/nonexistent", config.FileName))
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	if err := os.WriteFile(path, []byte("invalid = [[["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestContext(t *testing.T) {
	cfg := config.DefaultConfig("test", testDefaultBranch)
	ctx := config.WithConfig(context.Background(), cfg)
	got := config.FromContext(ctx)

	if got != cfg {
		t.Error("FromContext did not return the stored config")
	}
}

func TestFromContextNil(t *testing.T) {
	got := config.FromContext(context.Background())
	if got != nil {
		t.Error("FromContext on empty context should return nil")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
	}{
		{
			name: "fully populated config",
			cfg:  config.Config{WorktreeDir: "../worktrees", DefaultSource: testDefaultBranch},
		},
		{
			name: "empty worktree_dir is valid",
			cfg:  config.Config{DefaultSource: testDefaultBranch},
		},
		{
			name: "empty default_source is valid",
			cfg:  config.Config{WorktreeDir: "../worktrees"},
		},
		{
			name: "all empty is valid",
			cfg:  config.Config{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	// Valid TOML with only copy_files — empty worktree_dir/default_source is now valid
	if err := os.WriteFile(path, []byte("copy_files = ['.env']\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CopyFiles) != 1 || cfg.CopyFiles[0] != ".env" {
		t.Errorf("CopyFiles = %v, want [.env]", cfg.CopyFiles)
	}
	if cfg.WorktreeDir != "" {
		t.Errorf("WorktreeDir = %q, want empty", cfg.WorktreeDir)
	}
	if cfg.DefaultSource != "" {
		t.Errorf("DefaultSource = %q, want empty", cfg.DefaultSource)
	}
}

func TestSaveWriteError(t *testing.T) {
	cfg := config.DefaultConfig("test", testDefaultBranch)
	// Write to a path inside a nonexistent directory to trigger os.WriteFile error.
	err := config.Save("/nonexistent-dir/sub/config.toml", cfg)
	if err == nil {
		t.Fatal("expected error when writing to nonexistent directory")
	}
}

func TestSaveWriteErrorReadOnlyDir(t *testing.T) {
	dir := t.TempDir()

	// Make directory read-only
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	cfg := config.DefaultConfig("test", testDefaultBranch)
	err := config.Save(filepath.Join(dir, config.FileName), cfg)
	if err == nil {
		t.Fatal("expected error when writing to read-only directory")
	}
}

func TestSaveAndLoadWithOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	original := config.DefaultConfig(testRepoName, testDefaultBranch)
	original.Open = map[string]string{
		"ide":   "code .",
		"agent": "claude",
		"test":  "npm test",
	}
	if err := config.Save(path, original); err != nil {
		t.Fatalf(fatalSave, err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf(fatalLoad, err)
	}

	if !reflect.DeepEqual(original, loaded) {
		t.Errorf("loaded config differs:\n  got:  %+v\n  want: %+v", loaded, original)
	}
}

func TestLoadWithoutOpenSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	original := config.DefaultConfig(testRepoName, testDefaultBranch)
	if err := config.Save(path, original); err != nil {
		t.Fatalf(fatalSave, err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf(fatalLoad, err)
	}

	if loaded.Open != nil {
		t.Errorf("expected nil Open field, got %v", loaded.Open)
	}
}

// --- Merge tests ---

func TestMergeScalarOverrides(t *testing.T) {
	team := &config.Config{WorktreeDir: "../wt", DefaultSource: testDefaultBranch}
	local := &config.Config{WorktreeDir: "../local-wt", DefaultSource: testDevelopBranch}

	merged := config.Merge(team, local)
	if merged.WorktreeDir != "../local-wt" {
		t.Errorf("WorktreeDir = %q, want %q", merged.WorktreeDir, "../local-wt")
	}
	if merged.DefaultSource != testDevelopBranch {
		t.Errorf("DefaultSource = %q, want %q", merged.DefaultSource, testDevelopBranch)
	}
}

func TestMergeSliceReplaces(t *testing.T) {
	team := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: testDefaultBranch,
		CopyFiles:     []string{".env", ".envrc"},
		PostCreate:    []string{"make build"},
	}
	local := &config.Config{
		CopyFiles:  []string{".env.local"},
		PostCreate: []string{"npm install", "npm run build"},
	}

	merged := config.Merge(team, local)
	if !reflect.DeepEqual(merged.CopyFiles, []string{".env.local"}) {
		t.Errorf("CopyFiles = %v, want [.env.local]", merged.CopyFiles)
	}
	if !reflect.DeepEqual(merged.PostCreate, []string{"npm install", "npm run build"}) {
		t.Errorf("PostCreate = %v, want [npm install, npm run build]", merged.PostCreate)
	}
}

func TestMergeExplicitlyEmptySlice(t *testing.T) {
	team := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: testDefaultBranch,
		CopyFiles:     []string{".env"},
	}
	local := &config.Config{
		CopyFiles: []string{}, // explicitly empty — should override team
	}

	merged := config.Merge(team, local)
	if merged.CopyFiles == nil || len(merged.CopyFiles) != 0 {
		t.Errorf("CopyFiles = %v, want empty non-nil slice", merged.CopyFiles)
	}
}

func TestMergeMapReplaces(t *testing.T) {
	team := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: testDefaultBranch,
		Open:          map[string]string{"ide": "code ."},
	}
	local := &config.Config{
		Open: map[string]string{"ide": "vim", "test": "make test"},
	}

	merged := config.Merge(team, local)
	if !reflect.DeepEqual(merged.Open, local.Open) {
		t.Errorf("Open = %v, want %v", merged.Open, local.Open)
	}
}

func TestMergeDepsReplaces(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	team := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: testDefaultBranch,
		Deps:          &config.DepsConfig{AutoDetect: boolPtr(true)},
	}
	local := &config.Config{
		Deps: &config.DepsConfig{AutoDetect: boolPtr(false)},
	}

	merged := config.Merge(team, local)
	if merged.Deps == nil || merged.Deps.AutoDetect == nil || *merged.Deps.AutoDetect != false {
		t.Errorf("Deps.AutoDetect = %v, want false", merged.Deps)
	}
}

func TestMergeEmptyLocal(t *testing.T) {
	team := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: testDefaultBranch,
		CopyFiles:     []string{".env"},
	}

	merged := config.Merge(team, nil)
	if merged != team {
		t.Error("Merge with nil local should return team pointer")
	}
}

func TestMergeInputsNotMutated(t *testing.T) {
	team := &config.Config{
		WorktreeDir:   "../wt",
		DefaultSource: testDefaultBranch,
	}
	local := &config.Config{
		DefaultSource: testDevelopBranch,
	}

	_ = config.Merge(team, local)
	if team.DefaultSource != testDefaultBranch {
		t.Error("team config was mutated by Merge")
	}
}

// --- LoadDir tests ---

func TestLoadDirTeamOnly(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	team := config.DefaultConfig(testRepoName, testDefaultBranch)
	if err := config.Save(filepath.Join(rimbaDir, config.TeamFile), team); err != nil {
		t.Fatalf(fatalSave, err)
	}

	cfg, err := config.LoadDir(rimbaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if cfg.DefaultSource != testDefaultBranch {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, testDefaultBranch)
	}
}

func TestLoadDirTeamAndLocal(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	team := config.DefaultConfig(testRepoName, testDefaultBranch)
	if err := config.Save(filepath.Join(rimbaDir, config.TeamFile), team); err != nil {
		t.Fatalf(fatalSave, err)
	}

	local := &config.Config{DefaultSource: testDevelopBranch}
	if err := config.Save(filepath.Join(rimbaDir, config.LocalFile), local); err != nil {
		t.Fatalf(fatalSave, err)
	}

	cfg, err := config.LoadDir(rimbaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if cfg.DefaultSource != testDevelopBranch {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, testDevelopBranch)
	}
	if cfg.WorktreeDir != team.WorktreeDir {
		t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, team.WorktreeDir)
	}
}

func TestLoadDirTeamReadError(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create team file but make it unreadable
	teamPath := filepath.Join(rimbaDir, config.TeamFile)
	if err := os.WriteFile(teamPath, []byte("worktree_dir = \"../wt\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(teamPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(teamPath, 0644) })

	_, err := config.LoadDir(rimbaDir)
	if err == nil {
		t.Fatal("expected error when team config is unreadable")
	}
	if !strings.Contains(err.Error(), "failed to read team config") {
		t.Errorf("error = %q, want substring 'failed to read team config'", err.Error())
	}
}

func TestLoadDirMissingTeamFile(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := config.LoadDir(rimbaDir)
	if err == nil {
		t.Fatal("expected error when team config is missing")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error = %q, want substring 'does not exist'", err.Error())
	}
}

func TestLoadDirMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Minimal config with only copy_files — now valid (fields auto-derived later)
	if err := os.WriteFile(filepath.Join(rimbaDir, config.TeamFile), []byte("copy_files = ['.env']\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadDir(rimbaDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CopyFiles) != 1 || cfg.CopyFiles[0] != ".env" {
		t.Errorf("CopyFiles = %v, want [.env]", cfg.CopyFiles)
	}
}

func TestLoadDirInvalidLocalTOML(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	team := config.DefaultConfig(testRepoName, testDefaultBranch)
	if err := config.Save(filepath.Join(rimbaDir, config.TeamFile), team); err != nil {
		t.Fatalf(fatalSave, err)
	}

	if err := os.WriteFile(filepath.Join(rimbaDir, config.LocalFile), []byte("invalid = [[["), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.LoadDir(rimbaDir)
	if err == nil {
		t.Fatal("expected error for invalid local TOML")
	}
}

// --- Resolve tests ---

func TestResolveNewDirConfig(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	team := config.DefaultConfig(testRepoName, testDefaultBranch)
	if err := config.Save(filepath.Join(rimbaDir, config.TeamFile), team); err != nil {
		t.Fatalf(fatalSave, err)
	}

	cfg, err := config.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.DefaultSource != testDefaultBranch {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, testDefaultBranch)
	}
}

func TestResolveLegacyFallback(t *testing.T) {
	dir := t.TempDir()

	cfg := config.DefaultConfig(testRepoName, testDefaultBranch)
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf(fatalSave, err)
	}

	loaded, err := config.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if loaded.DefaultSource != testDefaultBranch {
		t.Errorf("DefaultSource = %q, want %q", loaded.DefaultSource, testDefaultBranch)
	}
}

func TestResolveDirTakesPrecedenceOverLegacy(t *testing.T) {
	dir := t.TempDir()

	// Create legacy config
	legacy := &config.Config{WorktreeDir: "../legacy-wt", DefaultSource: testDefaultBranch}
	if err := config.Save(filepath.Join(dir, config.FileName), legacy); err != nil {
		t.Fatalf(fatalSave, err)
	}

	// Create dir config
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}
	team := &config.Config{WorktreeDir: "../dir-wt", DefaultSource: testDefaultBranch}
	if err := config.Save(filepath.Join(rimbaDir, config.TeamFile), team); err != nil {
		t.Fatalf(fatalSave, err)
	}

	cfg, err := config.Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.WorktreeDir != "../dir-wt" {
		t.Errorf("WorktreeDir = %q, want %q (dir should take precedence)", cfg.WorktreeDir, "../dir-wt")
	}
}

func TestResolveNeitherExists(t *testing.T) {
	dir := t.TempDir()

	_, err := config.Resolve(dir)
	if err == nil {
		t.Fatal("expected error when neither config exists")
	}
}

func TestIsAutoDetectDeps(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name string
		cfg  config.Config
		want bool
	}{
		{
			name: "nil deps",
			cfg:  config.Config{WorktreeDir: "../wt", DefaultSource: testDefaultBranch},
			want: true,
		},
		{
			name: "nil auto_detect",
			cfg:  config.Config{WorktreeDir: "../wt", DefaultSource: testDefaultBranch, Deps: &config.DepsConfig{}},
			want: true,
		},
		{
			name: "auto_detect true",
			cfg:  config.Config{WorktreeDir: "../wt", DefaultSource: testDefaultBranch, Deps: &config.DepsConfig{AutoDetect: boolPtr(true)}},
			want: true,
		},
		{
			name: "auto_detect false",
			cfg:  config.Config{WorktreeDir: "../wt", DefaultSource: testDefaultBranch, Deps: &config.DepsConfig{AutoDetect: boolPtr(false)}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.IsAutoDetectDeps()
			if got != tt.want {
				t.Errorf("IsAutoDetectDeps() = %v, want %v", got, tt.want)
			}
		})
	}
}
