package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/lugassawan/rimba/internal/deps"
)

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

func TestBuildDepResults(t *testing.T) {
	results := []deps.InstallResult{
		{Module: deps.Module{Dir: "node_modules"}, Source: "/other/worktree", Cloned: true},
		{Module: deps.Module{Dir: "vendor"}, Error: errors.New("install failed")},
	}

	got := buildDepResults(results)
	if len(got) != 2 {
		t.Fatalf("buildDepResults() len = %d, want 2", len(got))
	}
	if got[0].Module != "node_modules" || !got[0].Cloned || got[0].Error != "" {
		t.Errorf("got[0] = %+v, want cloned node_modules with no error", got[0])
	}
	if got[1].Module != "vendor" || got[1].Cloned || got[1].Error != "install failed" {
		t.Errorf("got[1] = %+v, want vendor error 'install failed'", got[1])
	}

	if empty := buildDepResults(nil); empty == nil || len(empty) != 0 {
		t.Errorf("buildDepResults(nil) = %#v, want empty non-nil slice", empty)
	}
}

func TestBuildHookResults(t *testing.T) {
	results := []deps.HookResult{
		{Command: "make build"},
		{Command: "make test", Error: errors.New("exit 1")},
	}

	got := buildHookResults(results)
	if len(got) != 2 {
		t.Fatalf("buildHookResults() len = %d, want 2", len(got))
	}
	if got[0].Command != "make build" || got[0].Error != "" {
		t.Errorf("got[0] = %+v, want make build with no error", got[0])
	}
	if got[1].Command != "make test" || got[1].Error != "exit 1" {
		t.Errorf("got[1] = %+v, want make test error 'exit 1'", got[1])
	}

	if empty := buildHookResults(nil); empty == nil || len(empty) != 0 {
		t.Errorf("buildHookResults(nil) = %#v, want empty non-nil slice", empty)
	}
}
