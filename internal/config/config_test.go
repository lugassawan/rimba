package config_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig("myrepo", "main")

	if cfg.WorktreeDir != "../myrepo-worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", cfg.WorktreeDir, "../myrepo-worktrees")
	}
	if cfg.DefaultPrefix != "feat/" {
		t.Errorf("DefaultPrefix = %q, want %q", cfg.DefaultPrefix, "feat/")
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
	path := filepath.Join(dir, ".rimba.toml")

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
	_, err := config.Load("/nonexistent/.rimba.toml")
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".rimba.toml")

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
