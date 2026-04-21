package gh

import (
	"context"
	"errors"
	"testing"
)

const sameRepoPRJSON = `{
  "number": 42,
  "title": "Fix login redirect",
  "headRefName": "fix-login-redirect",
  "headRepository": {"name": "rimba"},
  "headRepositoryOwner": {"login": "lugassawan"},
  "isCrossRepository": false
}`

const crossForkPRJSON = `{
  "number": 99,
  "title": "Add OAuth support",
  "headRefName": "feat-oauth",
  "headRepository": {"name": "rimba"},
  "headRepositoryOwner": {"login": "contributor"},
  "isCrossRepository": true
}`

func TestFetchPRMetaSameRepo(t *testing.T) {
	r := &mockRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			return []byte(sameRepoPRJSON), nil
		},
	}
	meta, err := FetchPRMeta(context.Background(), r, 42)
	if err != nil {
		t.Fatalf("FetchPRMeta: %v", err)
	}
	if meta.Number != 42 {
		t.Errorf("Number = %d, want 42", meta.Number)
	}
	if meta.Title != "Fix login redirect" {
		t.Errorf("Title = %q, want %q", meta.Title, "Fix login redirect")
	}
	if meta.HeadRefName != "fix-login-redirect" {
		t.Errorf("HeadRefName = %q, want %q", meta.HeadRefName, "fix-login-redirect")
	}
	if meta.HeadRepoOwner != "lugassawan" {
		t.Errorf("HeadRepoOwner = %q, want %q", meta.HeadRepoOwner, "lugassawan")
	}
	if meta.IsCrossRepository {
		t.Error("IsCrossRepository should be false for same-repo PR")
	}
}

func TestFetchPRMetaCrossFork(t *testing.T) {
	r := &mockRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			return []byte(crossForkPRJSON), nil
		},
	}
	meta, err := FetchPRMeta(context.Background(), r, 99)
	if err != nil {
		t.Fatalf("FetchPRMeta: %v", err)
	}
	if !meta.IsCrossRepository {
		t.Error("IsCrossRepository should be true for cross-fork PR")
	}
	if meta.HeadRepoOwner != "contributor" {
		t.Errorf("HeadRepoOwner = %q, want %q", meta.HeadRepoOwner, "contributor")
	}
	if meta.HeadRepoName != "rimba" {
		t.Errorf("HeadRepoName = %q, want %q", meta.HeadRepoName, "rimba")
	}
}

func TestFetchPRMetaRunnerError(t *testing.T) {
	r := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return nil, errors.New("PR not found")
		},
	}
	_, err := FetchPRMeta(context.Background(), r, 999)
	if err == nil {
		t.Fatal("expected error when runner fails")
	}
	assertContains(t, err, "fetch PR #999")
}

func TestFetchPRMetaInvalidJSON(t *testing.T) {
	r := &mockRunner{
		run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte("not json"), nil
		},
	}
	_, err := FetchPRMeta(context.Background(), r, 1)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	assertContains(t, err, "parse PR #1")
}

func TestFetchPRMetaArgsIncludeNumber(t *testing.T) {
	var capturedArgs []string
	r := &mockRunner{
		run: func(_ context.Context, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte(sameRepoPRJSON), nil
		},
	}
	_, _ = FetchPRMeta(context.Background(), r, 42)
	if len(capturedArgs) < 3 || capturedArgs[1] != "view" || capturedArgs[2] != "42" {
		t.Errorf("expected args to include 'view 42', got %v", capturedArgs)
	}
}
