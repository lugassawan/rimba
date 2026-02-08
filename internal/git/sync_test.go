package git

import (
	"errors"
	"slices"
	"testing"
)

const (
	remoteOrigin    = "origin"
	branchMain      = "main"
	branchFeature   = "feature/test"
	fakeSHA         = "abc123"
	errRebaseFail   = "rebase failed"
	errFetchFail    = "fetch failed"
	flagRebaseAbort = "--abort"
)

func TestFetch(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}

	if err := Fetch(r, remoteOrigin); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(captured) != 2 || captured[0] != "fetch" || captured[1] != remoteOrigin {
		t.Errorf("expected [fetch origin], got %v", captured)
	}
}

func TestFetchError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errFetchFail)
		},
	}

	err := Fetch(r, remoteOrigin)
	if err == nil {
		t.Fatal("expected error from Fetch")
	}
	assertContains(t, err, errFetchFail)
}

func TestRebase(t *testing.T) {
	var capturedDir string
	var capturedArgs []string
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			capturedDir = dir
			capturedArgs = args
			return "", nil
		},
	}

	if err := Rebase(r, fakeDir, branchMain); err != nil {
		t.Fatalf("Rebase: %v", err)
	}

	if capturedDir != fakeDir {
		t.Errorf("dir = %q, want %q", capturedDir, fakeDir)
	}
	if len(capturedArgs) != 2 || capturedArgs[0] != "rebase" || capturedArgs[1] != branchMain {
		t.Errorf("expected [rebase main], got %v", capturedArgs)
	}
}

func TestRebaseError(t *testing.T) {
	r := &mockRunner{
		runInDir: func(_ string, _ ...string) (string, error) {
			return "", errors.New(errRebaseFail)
		},
	}

	err := Rebase(r, fakeDir, branchMain)
	if err == nil {
		t.Fatal("expected error from Rebase")
	}
	assertContains(t, err, errRebaseFail)
}

func TestAbortRebase(t *testing.T) {
	var capturedDir string
	var capturedArgs []string
	r := &mockRunner{
		runInDir: func(dir string, args ...string) (string, error) {
			capturedDir = dir
			capturedArgs = args
			return "", nil
		},
	}

	if err := AbortRebase(r, fakeDir); err != nil {
		t.Fatalf("AbortRebase: %v", err)
	}

	if capturedDir != fakeDir {
		t.Errorf("dir = %q, want %q", capturedDir, fakeDir)
	}
	if !slices.Contains(capturedArgs, flagRebaseAbort) {
		t.Errorf(errExpectedInFmt, flagRebaseAbort, capturedArgs)
	}
}

func TestMergeBase(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) == 3 && args[0] == "merge-base" {
				return fakeSHA, nil
			}
			return "", errors.New("unexpected")
		},
	}

	sha, err := MergeBase(r, branchMain, branchFeature)
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if sha != fakeSHA {
		t.Errorf("sha = %q, want %q", sha, fakeSHA)
	}
}

func TestMergeBaseError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New(errNotARepo)
		},
	}

	_, err := MergeBase(r, branchMain, branchFeature)
	if err == nil {
		t.Fatal("expected error from MergeBase")
	}
}

func TestIsMergeBaseAncestor(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if slices.Contains(args, "--is-ancestor") {
				return "", nil
			}
			return "", errors.New("unexpected")
		},
	}

	if !IsMergeBaseAncestor(r, branchMain, branchFeature) {
		t.Error("expected true for ancestor check")
	}
}

func TestIsMergeBaseAncestorFalse(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New("not ancestor")
		},
	}

	if IsMergeBaseAncestor(r, branchMain, branchFeature) {
		t.Error("expected false for non-ancestor")
	}
}
