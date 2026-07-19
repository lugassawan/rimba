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
			{Module: deps.Module{Dir: "node_modules"}, Ran: true},
		}
		printInstallResults(buf, results)
		if buf.Len() != 0 {
			t.Errorf("expected no output for no-op results, got %q", buf.String())
		}
	})

	t.Run("cancelled module", func(t *testing.T) {
		buf := new(bytes.Buffer)
		results := []deps.InstallResult{
			{Module: deps.Module{Dir: "node_modules"}, Ran: false},
		}
		printInstallResults(buf, results)
		out := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("skipped (cancelled)")) {
			t.Errorf("output missing 'skipped (cancelled)': %q", out)
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

	t.Run("deferred module", func(t *testing.T) {
		buf := new(bytes.Buffer)
		results := []deps.InstallResult{
			{Module: deps.Module{Dir: "node_modules"}, Deferred: true},
		}
		printInstallResults(buf, results)
		out := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("Dependencies:")) {
			t.Errorf("output missing 'Dependencies:' header: %q", out)
		}
		if !bytes.Contains(buf.Bytes(), []byte("deferred")) {
			t.Errorf("output missing 'deferred': %q", out)
		}
		if !bytes.Contains(buf.Bytes(), []byte("rimba deps install")) {
			t.Errorf("output missing actionable hint: %q", out)
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

func TestInstallResultLineDeferred(t *testing.T) {
	res := deps.InstallResult{
		Module:   deps.Module{Dir: "node_modules"},
		Deferred: true,
	}
	got := installResultLine(res)
	want := "node_modules: deferred — run `rimba deps install <task> --path node_modules` if you need it"
	if got != want {
		t.Errorf("installResultLine() = %q, want %q", got, want)
	}
}

func TestBuildDepResults(t *testing.T) {
	results := []deps.InstallResult{
		{Module: deps.Module{Dir: "node_modules"}, Source: "/other/worktree", Cloned: true, Ran: true},
		{Module: deps.Module{Dir: "vendor"}, Error: errors.New("install failed"), Ran: true},
	}

	got := buildDepResults(results)
	if len(got) != 2 {
		t.Fatalf("buildDepResults() len = %d, want 2", len(got))
	}
	if got[0].Module != "node_modules" || !got[0].Cloned || got[0].Error != "" || !got[0].Ran {
		t.Errorf("got[0] = %+v, want cloned node_modules with no error, Ran=true", got[0])
	}
	if got[1].Module != "vendor" || got[1].Cloned || got[1].Error != "install failed" || !got[1].Ran {
		t.Errorf("got[1] = %+v, want vendor error 'install failed', Ran=true", got[1])
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
