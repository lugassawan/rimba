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

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig("myrepo", "main")

	if cfg.WorktreeDir != "../myrepo-worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, "../myrepo-worktrees")
	}
	if cfg.DefaultSource != "main" {
		t.Errorf("DefaultSource = %q, want %q", cfg.DefaultSource, "main")
	}
	expected := []string{".env", ".env.local", ".envrc", ".tool-versions"}
	if !reflect.DeepEqual(cfg.CopyFiles, expected) {
		t.Errorf("CopyFiles = %v, want %v", cfg.CopyFiles, expected)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	original := config.DefaultConfig("test-repo", "main")
	if err := config.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
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
	cfg := config.DefaultConfig("test", "main")
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
			cfg:  config.Config{WorktreeDir: "../worktrees", DefaultSource: "main"},
		},
		{
			name:    "empty worktree_dir",
			cfg:     config.Config{DefaultSource: "main"},
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
	cfg := config.DefaultConfig("test", "main")
	// Write to a path inside a nonexistent directory to trigger os.WriteFile error.
	err := config.Save("/nonexistent-dir/sub/config.toml", cfg)
	if err == nil {
		t.Fatal("expected error when writing to nonexistent directory")
	}
}
