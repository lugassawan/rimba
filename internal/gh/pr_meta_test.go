package gh

import (
	"context"
	"errors"
	"testing"

	"github.com/lugassawan/rimba/testutil"
)

func TestFetchPRMetaSameRepo(t *testing.T) {
	sameRepoPRJSON := testutil.LoadFixture(t, "testdata/same_repo_pr.json")
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
	crossForkPRJSON := testutil.LoadFixture(t, "testdata/cross_fork_pr.json")
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
	assertContains(t, err, "To fix:")
}

func TestFetchPRMetaRunnerErrorClassifier(t *testing.T) {
	tests := []struct {
		name       string
		runErr     string
		wantSubstr string
	}{
		{
			name:       "404 error routes to PR number hint",
			runErr:     "HTTP 404: Not Found",
			wantSubstr: "verify PR number and repo access",
		},
		{
			name:       "could not resolve routes to PR number hint",
			runErr:     "Could not resolve to a Repository",
			wantSubstr: "verify PR number and repo access",
		},
		{
			name:       "rate limit routes to token hint",
			runErr:     "API rate limit exceeded",
			wantSubstr: "GitHub API rate limit hit",
		},
		{
			name:       "generic error routes to auth hint",
			runErr:     "connection refused",
			wantSubstr: "gh auth status",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{
				run: func(_ context.Context, _ ...string) ([]byte, error) {
					return nil, errors.New(tt.runErr)
				},
			}
			_, err := FetchPRMeta(context.Background(), r, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			assertContains(t, err, tt.wantSubstr)
		})
	}
}

func TestFetchPRMetaShapeErrorsHaveGhHint(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"empty response", `{}`},
		{"missing owner", `{"headRefName":"main","headRepository":{"name":"repo"},"headRepositoryOwner":{"login":""}}`},
		{"missing repo", `{"headRefName":"main","headRepository":{"name":""},"headRepositoryOwner":{"login":"alice"}}`},
		{"unsafe owner", `{"headRefName":"main","headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"alice;rm -rf /"}}`},
		{"unsafe repo", `{"headRefName":"main","headRepository":{"name":"repo$(whoami)"},"headRepositoryOwner":{"login":"alice"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{
				run: func(_ context.Context, _ ...string) ([]byte, error) {
					return []byte(tt.json), nil
				},
			}
			_, err := FetchPRMeta(context.Background(), r, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			assertContains(t, err, "To fix:")
			assertContains(t, err, "update gh")
		})
	}
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

func TestFetchPRMetaEmptyFields(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name:    "empty response",
			json:    `{}`,
			wantErr: "missing headRefName",
		},
		{
			name:    "missing owner",
			json:    `{"headRefName":"main","headRepository":{"name":"repo"},"headRepositoryOwner":{"login":""}}`,
			wantErr: "missing headRepositoryOwner",
		},
		{
			name:    "missing repo name",
			json:    `{"headRefName":"main","headRepository":{"name":""},"headRepositoryOwner":{"login":"alice"}}`,
			wantErr: "missing headRepository",
		},
		{
			name:    "unsafe owner characters",
			json:    `{"headRefName":"main","headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"alice;rm -rf /"}}`,
			wantErr: "unsafe headRepositoryOwner",
		},
		{
			name:    "unsafe repo name characters",
			json:    `{"headRefName":"main","headRepository":{"name":"repo$(whoami)"},"headRepositoryOwner":{"login":"alice"}}`,
			wantErr: "unsafe headRepository.name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &mockRunner{
				run: func(_ context.Context, _ ...string) ([]byte, error) {
					return []byte(tt.json), nil
				},
			}
			_, err := FetchPRMeta(context.Background(), r, 1)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			assertContains(t, err, tt.wantErr)
		})
	}
}

func TestFetchPRMetaArgsIncludeNumber(t *testing.T) {
	sameRepoPRJSON := testutil.LoadFixture(t, "testdata/same_repo_pr.json")
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

func TestFetchPRMetaHeadRefNameValidation(t *testing.T) {
	makeJSON := func(ref string) string {
		return `{"headRefName":"` + ref + `","headRepository":{"name":"rimba"},"headRepositoryOwner":{"login":"alice"}}`
	}

	valid := []string{
		"feature/foo",
		"dependabot/go_modules/x",
		"release-1.2",
		"fix-login-redirect",
		"main",
	}
	for _, ref := range valid {
		t.Run("accept "+ref, func(t *testing.T) {
			r := &mockRunner{
				run: func(_ context.Context, _ ...string) ([]byte, error) {
					return []byte(makeJSON(ref)), nil
				},
			}
			_, err := FetchPRMeta(context.Background(), r, 1)
			if err != nil {
				t.Errorf("expected valid branch %q to be accepted, got error: %v", ref, err)
			}
		})
	}

	invalid := []struct {
		ref     string
		wantErr string
	}{
		{"-foo", "leading dash"},
		{"../etc/passwd", "contains .."},
		{"/abs/path", "leading slash"},
		{"a;b", "unsafe git ref name"},
		{"a b", "unsafe git ref name"},
	}
	for _, tt := range invalid {
		t.Run("reject "+tt.ref, func(t *testing.T) {
			r := &mockRunner{
				run: func(_ context.Context, _ ...string) ([]byte, error) {
					return []byte(makeJSON(tt.ref)), nil
				},
			}
			_, err := FetchPRMeta(context.Background(), r, 1)
			if err == nil {
				t.Fatalf("expected error for branch %q, got nil", tt.ref)
			}
			assertContains(t, err, tt.wantErr)
			assertContains(t, err, "To fix:")
		})
	}
}
