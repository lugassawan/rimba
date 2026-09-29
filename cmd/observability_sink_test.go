package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestOpenObservabilitySinkNilWhenNotOpened(t *testing.T) {
	disabled := false
	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{"nil config", nil},
		{"disabled", &config.Config{Observability: &config.ObservabilityConfig{Enabled: &disabled}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := redirectCacheDir(t)
			if sink := openObservabilitySink(tt.cfg, t.TempDir()); sink != nil {
				t.Errorf("sink = %v, want nil", sink)
			}
			if files := findCacheJSONLFiles(t, home); len(files) != 0 {
				t.Errorf("expected zero filesystem footprint, got %v", files)
			}
		})
	}
}

func TestOpenObservabilitySinkEnabledOpensDayFiles(t *testing.T) {
	home := redirectCacheDir(t)
	sink := openObservabilitySink(&config.Config{}, t.TempDir())
	if sink == nil {
		t.Fatal("expected a sink when observability is enabled")
	}
	if err := sink.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if files := findCacheJSONLFiles(t, home); len(files) != 2 {
		t.Errorf("day files = %v, want log + metrics", files)
	}
}

func TestOpenObservabilitySinkOpenFailureReturnsNil(t *testing.T) {
	// A regular file as HOME makes the cache dir uncreatable.
	homeFile := filepath.Join(t.TempDir(), "home-is-a-file")
	if err := os.WriteFile(homeFile, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("HOME", homeFile)
	t.Setenv("XDG_CACHE_HOME", "")
	os.Unsetenv("XDG_CACHE_HOME")
	t.Setenv("RIMBA_DEBUG", "1")

	if sink := openObservabilitySink(&config.Config{}, t.TempDir()); sink != nil {
		t.Errorf("sink = %v, want nil when opening fails", sink)
	}
}
