package operations

import (
	"context"
	"testing"
)

func newNoopRunner() *mockRunner {
	return &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}
}

func TestPostRenameSetupSkipAll(t *testing.T) {
	params := PostRenameParams{
		WtPath:     "/wt/feature/auth",
		SkipDeps:   true,
		SkipHooks:  true,
		PostRename: [][]string{{"echo hook"}},
	}
	res, err := PostRenameSetup(context.Background(), newNoopRunner(), params, nil)
	if err != nil {
		t.Fatalf("PostRenameSetup: %v", err)
	}
	if len(res.DepsResults) != 0 {
		t.Errorf("expected no deps results when SkipDeps=true, got %d", len(res.DepsResults))
	}
	if len(res.HookResults) != 0 {
		t.Errorf("expected no hook results when SkipHooks=true, got %d", len(res.HookResults))
	}
}

func TestPostRenameSetupRunsHooks(t *testing.T) {
	hookDir := t.TempDir()
	params := PostRenameParams{
		WtPath:     hookDir,
		SkipDeps:   true,
		SkipHooks:  false,
		PostRename: [][]string{{"echo hook-ran"}},
	}
	res, err := PostRenameSetup(context.Background(), newNoopRunner(), params, nil)
	if err != nil {
		t.Fatalf("PostRenameSetup: %v", err)
	}
	if len(res.HookResults) != 1 {
		t.Fatalf("expected 1 hook result, got %d", len(res.HookResults))
	}
	if res.HookResults[0].Error != nil {
		t.Errorf("hook error = %v, want nil", res.HookResults[0].Error)
	}
}

func TestPostRenameSetupNoHooksWhenEmpty(t *testing.T) {
	params := PostRenameParams{
		WtPath:     "/wt/feature/auth",
		SkipDeps:   true,
		SkipHooks:  false,
		PostRename: nil,
	}
	res, err := PostRenameSetup(context.Background(), newNoopRunner(), params, nil)
	if err != nil {
		t.Fatalf("PostRenameSetup: %v", err)
	}
	if len(res.HookResults) != 0 {
		t.Errorf("expected no hook results when PostRename is empty, got %d", len(res.HookResults))
	}
}

func TestPostRenameSetupHookFailure(t *testing.T) {
	hookDir := t.TempDir()
	params := PostRenameParams{
		WtPath:     hookDir,
		SkipDeps:   true,
		SkipHooks:  false,
		PostRename: [][]string{{"false"}},
	}
	res, err := PostRenameSetup(context.Background(), newNoopRunner(), params, nil)
	if err != nil {
		t.Fatalf("PostRenameSetup: %v", err)
	}
	if len(res.HookResults) != 1 {
		t.Fatalf("expected 1 hook result, got %d", len(res.HookResults))
	}
	if res.HookResults[0].Error == nil {
		t.Error("expected hook failure, got nil error")
	}
}

func TestPostRenameSetupDepsNoop(t *testing.T) {
	// Non-existent path: deps.ResolveModules returns nil → no-op
	params := PostRenameParams{
		WtPath:     "/nonexistent/path/that/does/not/exist",
		SkipDeps:   false,
		AutoDetect: true,
		SkipHooks:  true,
	}
	res, err := PostRenameSetup(context.Background(), newNoopRunner(), params, nil)
	if err != nil {
		t.Fatalf("PostRenameSetup: %v", err)
	}
	if len(res.DepsResults) != 0 {
		t.Errorf("expected no deps results for non-existent path, got %d", len(res.DepsResults))
	}
}
