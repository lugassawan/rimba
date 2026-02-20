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
		name    string
		cfg     config.Config
		wantErr string
	}{
		{
			name: "valid config",
			cfg:  config.Config{WorktreeDir: "../worktrees", DefaultSource: testDefaultBranch},
		},
		{
			name:    "empty worktree_dir",
			cfg:     config.Config{DefaultSource: testDefaultBranch},
			wantErr: config.ErrMsgEmptyWorktreeDir,
		},
		{
			name:    "empty default_source",
			cfg:     config.Config{WorktreeDir: "../worktrees"},
			wantErr: config.ErrMsgEmptyDefaultSource,
		},
		{
			name:    "both empty",
			cfg:     config.Config{},
			wantErr: config.ErrMsgEmptyWorktreeDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadInvalidValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	// Valid TOML but missing required fields
	if err := os.WriteFile(path, []byte("copy_files = ['.env']\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
	if !strings.Contains(err.Error(), config.ErrMsgEmptyWorktreeDir) {
		t.Errorf("error %q does not mention worktree_dir", err.Error())
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
	local := &config.Config{DefaultSource: "develop"}

	merged := config.Merge(team, local)
	if merged.WorktreeDir != "../wt" {
		t.Errorf("WorktreeDir = %q, want %q", merged.WorktreeDir, "../wt")
	}
	if merged.DefaultSource != "develop" {
		t.Errorf("DefaultSource = %q, want %q", merged.DefaultSource, "develop")
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
		CopyFiles: []string{".env.local"},
	}

	merged := config.Merge(team, local)
	if !reflect.DeepEqual(merged.CopyFiles, []string{".env.local"}) {
		t.Errorf("CopyFiles = %v, want [.env.local]", merged.CopyFiles)
	}
	// PostCreate should be preserved from team (local is nil)
	if !reflect.DeepEqual(merged.PostCreate, []string{"make build"}) {
		t.Errorf("PostCreate = %v, want [make build]", merged.PostCreate)
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
		DefaultSource: "develop",
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

	local := &config.Config{DefaultSource: "develop"}
	if err := config.Save(filepath.Join(rimbaDir, config.LocalFile), local); err != nil {
		t.Fatalf(fatalSave, err)
	}

	cfg, err := config.LoadDir(rimbaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if cfg.DefaultSource != "develop" {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, "develop")
	}
	if cfg.WorktreeDir != team.WorktreeDir {
		t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, team.WorktreeDir)
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

func TestLoadDirValidationError(t *testing.T) {
	dir := t.TempDir()
	rimbaDir := filepath.Join(dir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write invalid config (missing required fields)
	if err := os.WriteFile(filepath.Join(rimbaDir, config.TeamFile), []byte("copy_files = ['.env']\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.LoadDir(rimbaDir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), config.ErrMsgEmptyWorktreeDir) {
		t.Errorf("error = %q, want substring %q", err.Error(), config.ErrMsgEmptyWorktreeDir)
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
