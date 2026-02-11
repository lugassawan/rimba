package cmd

import (
	"bytes"
	"errors"
	"testing"

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
