package git

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestListIgnoredUntrackedArgs(t *testing.T) {
	var capturedDir string
	var capturedArgs []string
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			capturedDir = dir
			capturedArgs = args
			return "", nil
		},
	}

	pathspecs := []string{".env", ".claude"}
	if _, err := ListIgnoredUntracked(context.Background(), r, fakeDir, pathspecs); err != nil {
		t.Fatalf("ListIgnoredUntracked: %v", err)
	}

	want := []string{"ls-files", "--others", "--ignored", "--exclude-standard", "--", ".env", ".claude"}
	if !slices.Equal(capturedArgs, want) {
		t.Errorf("args = %v, want %v", capturedArgs, want)
	}
	if capturedDir != fakeDir {
		t.Errorf("dir = %q, want %q", capturedDir, fakeDir)
	}
}

func TestListIgnoredUntrackedParsesOutput(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return ".env\n.claude/settings.local.json", nil
		},
	}

	got, err := ListIgnoredUntracked(context.Background(), r, fakeDir, []string{".env", ".claude"})
	if err != nil {
		t.Fatalf("ListIgnoredUntracked: %v", err)
	}

	want := []string{".env", ".claude/settings.local.json"}
	if !slices.Equal(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

func TestListIgnoredUntrackedEmptyOutput(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", nil
		},
	}

	got, err := ListIgnoredUntracked(context.Background(), r, fakeDir, []string{".env"})
	if err != nil {
		t.Fatalf("ListIgnoredUntracked: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestListIgnoredUntrackedError(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := ListIgnoredUntracked(context.Background(), r, fakeDir, []string{".env"})
	if err == nil {
		t.Fatal("expected error from ListIgnoredUntracked")
	}
	assertContains(t, err, errNotARepo)
}
