package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
)

func TestWorktreePathsExcluding(t *testing.T) {
	tests := []struct {
		name    string
		entries []git.WorktreeEntry
		exclude string
		want    int
	}{
		{
			name:    "empty",
			entries: nil,
			exclude: "/a",
			want:    0,
		},
		{
			name: "no match",
			entries: []git.WorktreeEntry{
				{Path: "/a"},
				{Path: "/b"},
			},
			exclude: "/c",
			want:    2,
		},
		{
			name: "with exclusion",
			entries: []git.WorktreeEntry{
				{Path: "/a"},
				{Path: "/b"},
				{Path: "/c"},
			},
			exclude: "/b",
			want:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worktreePathsExcluding(tt.entries, tt.exclude)
			if len(got) != tt.want {
				t.Errorf("worktreePathsExcluding() returned %d paths, want %d", len(got), tt.want)
			}
			for _, p := range got {
				if p == tt.exclude {
					t.Errorf("worktreePathsExcluding() included excluded path %q", tt.exclude)
				}
			}
		})
	}
}

func TestPrintInstallResults(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		buf := new(bytes.Buffer)
		printInstallResults(buf, nil)
		if buf.Len() != 0 {
			t.Errorf("expected no output for nil results, got %q", buf.String())
		}
	})

	t.Run("no clones no errors", func(t *testing.T) {
		buf := new(bytes.Buffer)
		results := []deps.InstallResult{
			{Module: deps.Module{Dir: "node_modules"}},
		}
		printInstallResults(buf, results)
		if buf.Len() != 0 {
			t.Errorf("expected no output for skipped results, got %q", buf.String())
		}
	})

	t.Run("cloned module", func(t *testing.T) {
		buf := new(bytes.Buffer)
		results := []deps.InstallResult{
			{Module: deps.Module{Dir: "node_modules"}, Source: "/other/worktree", Cloned: true},
		}
		printInstallResults(buf, results)
		out := buf.String()
		if out == "" {
			t.Fatal("expected output for cloned module")
		}
		if !bytes.Contains(buf.Bytes(), []byte("Dependencies:")) {
			t.Errorf("output missing 'Dependencies:' header: %q", out)
		}
		if !bytes.Contains(buf.Bytes(), []byte("cloned from")) {
			t.Errorf("output missing 'cloned from': %q", out)
		}
	})

	t.Run("error module", func(t *testing.T) {
		buf := new(bytes.Buffer)
		results := []deps.InstallResult{
			{Module: deps.Module{Dir: "vendor"}, Error: errors.New("install failed")},
		}
		printInstallResults(buf, results)
		out := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("Dependencies:")) {
			t.Errorf("output missing 'Dependencies:' header: %q", out)
		}
		if !bytes.Contains(buf.Bytes(), []byte("install failed")) {
			t.Errorf("output missing error message: %q", out)
		}
	})
}

func TestInstallDeps(t *testing.T) {
	existingWT := t.TempDir()
	newWT := t.TempDir()

	// Setup lockfiles and node_modules in existing worktree
	_ = os.WriteFile(filepath.Join(existingWT, deps.LockfilePnpm), []byte("lockfile-v6-content"), 0644)
	_ = os.WriteFile(filepath.Join(newWT, deps.LockfilePnpm), []byte("lockfile-v6-content"), 0644)
	_ = os.MkdirAll(filepath.Join(existingWT, deps.DirNodeModules), 0755)
	_ = os.WriteFile(filepath.Join(existingWT, deps.DirNodeModules, "package.json"), []byte("{}"), 0644)

	porcelain := strings.Join([]string{
		wtPrefix + existingWT,
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		wtPrefix + newWT,
		"HEAD def456",
		"branch refs/heads/feature/test",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	cfg := &config.Config{
		Deps: &config.DepsConfig{AutoDetect: boolPtr(true)},
	}

	entries := []git.WorktreeEntry{
		{Path: existingWT, Branch: "main"},
		{Path: newWT, Branch: "feature/test"},
	}

	results := installDeps(r, cfg, newWT, entries)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from installDeps")
	}
	// Should have cloned from existing worktree
	found := false
	for _, res := range results {
		if res.Cloned {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one cloned module")
	}
}

func TestInstallDepsNoLockfiles(t *testing.T) {
	newWT := t.TempDir()

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	cfg := &config.Config{
		Deps: &config.DepsConfig{AutoDetect: boolPtr(true)},
	}

	results := installDeps(r, cfg, newWT, nil)
	if results != nil {
		t.Errorf("expected nil results for no lockfiles, got %v", results)
	}
}

func TestInstallDepsPreferSource(t *testing.T) {
	sourceWT := t.TempDir()
	newWT := t.TempDir()

	_ = os.WriteFile(filepath.Join(sourceWT, deps.LockfilePnpm), []byte("lockfile-content"), 0644)
	_ = os.WriteFile(filepath.Join(newWT, deps.LockfilePnpm), []byte("lockfile-content"), 0644)
	_ = os.MkdirAll(filepath.Join(sourceWT, deps.DirNodeModules), 0755)
	_ = os.WriteFile(filepath.Join(sourceWT, deps.DirNodeModules, "origin.txt"), []byte(sourceWT), 0644)

	porcelain := strings.Join([]string{
		wtPrefix + sourceWT,
		"HEAD abc123",
		"branch refs/heads/feature/source",
		"",
		wtPrefix + newWT,
		"HEAD def456",
		"branch refs/heads/feature/copy",
		"",
	}, "\n")

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return porcelain, nil },
		runInDir: noopRunInDir,
	}

	cfg := &config.Config{
		Deps: &config.DepsConfig{AutoDetect: boolPtr(true)},
	}

	entries := []git.WorktreeEntry{
		{Path: sourceWT, Branch: "feature/source"},
		{Path: newWT, Branch: "feature/copy"},
	}

	results := installDepsPreferSource(r, cfg, newWT, sourceWT, entries)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if !results[0].Cloned {
		t.Error("expected module to be cloned from source")
	}
	if results[0].Source != sourceWT {
		t.Errorf("source = %q, want %q", results[0].Source, sourceWT)
	}
}

func TestInstallDepsPreferSourceNoLockfiles(t *testing.T) {
	newWT := t.TempDir()
	sourceWT := t.TempDir()

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	cfg := &config.Config{
		Deps: &config.DepsConfig{AutoDetect: boolPtr(true)},
	}

	results := installDepsPreferSource(r, cfg, newWT, sourceWT, nil)
	if results != nil {
		t.Errorf("expected nil results for no lockfiles, got %v", results)
	}
}

func TestInstallDepsNilDepsConfig(t *testing.T) {
	newWT := t.TempDir()

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	cfg := &config.Config{} // Deps is nil

	results := installDeps(r, cfg, newWT, nil)
	if results != nil {
		t.Errorf("expected nil results for nil Deps config, got %v", results)
	}
}

func TestInstallDepsPreferSourceNilDepsConfig(t *testing.T) {
	newWT := t.TempDir()
	sourceWT := t.TempDir()

	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	cfg := &config.Config{} // Deps is nil

	results := installDepsPreferSource(r, cfg, newWT, sourceWT, nil)
	if results != nil {
		t.Errorf("expected nil results for nil Deps config, got %v", results)
	}
}

func TestRunHooks(t *testing.T) {
	dir := t.TempDir()

	results := runHooks(dir, []string{"touch marker.txt"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("expected no error, got %v", results[0].Error)
	}

	if _, err := os.Stat(filepath.Join(dir, "marker.txt")); os.IsNotExist(err) {
		t.Error("expected marker.txt to exist")
	}
}

func TestRunHooksFailure(t *testing.T) {
	dir := t.TempDir()

	results := runHooks(dir, []string{"false"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error for 'false' command")
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestPrintHookResultsList(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		buf := new(bytes.Buffer)
		printHookResultsList(buf, nil)
		if buf.Len() != 0 {
			t.Errorf("expected no output for nil results, got %q", buf.String())
		}
	})

	t.Run("ok hook", func(t *testing.T) {
		buf := new(bytes.Buffer)
		results := []deps.HookResult{
			{Command: "make build"},
		}
		printHookResultsList(buf, results)
		out := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("Hooks:")) {
			t.Errorf("output missing 'Hooks:' header: %q", out)
		}
		if !bytes.Contains(buf.Bytes(), []byte("make build: ok")) {
			t.Errorf("output missing hook ok line: %q", out)
		}
	})

	t.Run("error hook", func(t *testing.T) {
		buf := new(bytes.Buffer)
		results := []deps.HookResult{
			{Command: "make test", Error: errors.New("exit 1")},
		}
		printHookResultsList(buf, results)
		out := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("make test:")) {
			t.Errorf("output missing hook command: %q", out)
		}
		if !bytes.Contains(buf.Bytes(), []byte("exit 1")) {
			t.Errorf("output missing error message: %q", out)
		}
	})
}
