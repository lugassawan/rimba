package config_test

import (
	"reflect"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestCandidateCopyFiles(t *testing.T) {
	files, dirs := config.CandidateCopyFiles()

	wantFiles := []string{
		".env", ".env.local",
		".env.development", ".env.development.local",
		".env.production", ".env.production.local",
		".env.test", ".env.test.local",
		".envrc", ".tool-versions", ".python-version", ".dev.vars", ".npmrc",
	}
	if !reflect.DeepEqual(files, wantFiles) {
		t.Errorf("files = %v, want %v", files, wantFiles)
	}

	wantDirs := []string{".vscode", ".idea", ".cursor", ".claude"}
	if !reflect.DeepEqual(dirs, wantDirs) {
		t.Errorf("dirs = %v, want %v", dirs, wantDirs)
	}
}

func TestDetectCopyFiles(t *testing.T) {
	tests := []struct {
		name    string
		ignored []string
		want    []string
	}{
		{
			name:    "file exact match",
			ignored: []string{".env"},
			want:    []string{".env"},
		},
		{
			name:    "non-.local env tier variants match",
			ignored: []string{".env.production", ".env.test"},
			want:    []string{".env.production", ".env.test"},
		},
		{
			name:    "dir prefix match",
			ignored: []string{".claude/settings.local.toml"},
			want:    []string{".claude"},
		},
		{
			name:    "unmatched candidate not present",
			ignored: []string{"some/other/path"},
			want:    nil,
		},
		{
			name:    "empty input",
			ignored: nil,
			want:    nil,
		},
		{
			name:    "order follows candidate order, not input order",
			ignored: []string{".npmrc", ".env", ".idea/workspace.xml"},
			want:    []string{".env", ".npmrc", ".idea"},
		},
		{
			name:    "dedup: multiple ignored files under same dir yield one entry",
			ignored: []string{".vscode/settings.json", ".vscode/launch.json"},
			want:    []string{".vscode"},
		},
		{
			name:    "dedup: duplicate exact-match ignored paths yield one entry",
			ignored: []string{".env", ".env"},
			want:    []string{".env"},
		},
		{
			name:    "file candidate does not match a similarly-named nested path",
			ignored: []string{"nested/.env"},
			want:    nil,
		},
		{
			name:    "mixed files and dirs preserve full candidate order",
			ignored: []string{".dev.vars", ".envrc", ".cursor/rules.mdc", ".claude/settings.local.toml"},
			want:    []string{".envrc", ".dev.vars", ".cursor", ".claude"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.DetectCopyFiles(tt.ignored)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DetectCopyFiles(%v) = %v, want %v", tt.ignored, got, tt.want)
			}
		})
	}
}
