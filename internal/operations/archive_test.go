package operations

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestArchiveWorktreeSuccess(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", nil },
		runInDir: noopRunInDir,
	}

	result, err := ArchiveWorktree(context.Background(), r, ArchiveParams{
		Path:   pathWtFeatureLogin,
		Branch: branchFeature,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != pathWtFeatureLogin {
		t.Errorf("path=%q, want %q", result.Path, pathWtFeatureLogin)
	}
	if result.Branch != branchFeature {
		t.Errorf("branch=%q, want %q", result.Branch, branchFeature)
	}
	if result.Plan == nil {
		t.Fatal("expected Plan to be non-nil")
	}
	if result.Plan.DryRun {
		t.Error("expected Plan.DryRun=false for real run")
	}
}

func TestArchiveWorktreeDryRun(t *testing.T) {
	gitCalled := false
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			gitCalled = true
			return "", nil
		},
		runInDir: noopRunInDir,
	}

	result, err := ArchiveWorktree(context.Background(), r, ArchiveParams{
		Path:   pathWtFeatureLogin,
		Branch: branchFeature,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gitCalled {
		t.Error("git must not be called in dry-run mode")
	}
	if result.Plan == nil || !result.Plan.DryRun {
		t.Fatal("expected Plan.DryRun=true")
	}
	if len(result.Plan.Steps) == 0 {
		t.Fatal("expected at least one planned step")
	}
	if !strings.Contains(result.Plan.Steps[0], pathWtFeatureLogin) {
		t.Errorf("step should mention worktree path, got: %v", result.Plan.Steps)
	}
}

func TestArchiveWorktreeError(t *testing.T) {
	r := &mockRunner{
		run:      func(_ ...string) (string, error) { return "", errors.New("locked") },
		runInDir: noopRunInDir,
	}

	_, err := ArchiveWorktree(context.Background(), r, ArchiveParams{
		Path:   pathWtFeatureLogin,
		Branch: branchFeature,
	})
	if err == nil {
		t.Fatal("expected error from git failure")
	}
}
