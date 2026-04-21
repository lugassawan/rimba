package git

import (
	"errors"
	"testing"
)

func TestRemoteExistsTrue(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "https://github.com/owner/repo.git", nil
		},
	}
	if !RemoteExists(r, "origin") {
		t.Error("expected remote to exist")
	}
}

func TestRemoteExistsFalse(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("no such remote 'gh-fork-bob'")
		},
	}
	if RemoteExists(r, "gh-fork-bob") {
		t.Error("expected remote to not exist")
	}
}

func TestAddRemote(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}
	if err := AddRemote(r, "gh-fork-alice", "https://github.com/alice/repo.git"); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	want := []string{"remote", "add", "gh-fork-alice", "https://github.com/alice/repo.git"}
	if len(captured) != len(want) {
		t.Fatalf("args = %v, want %v", captured, want)
	}
	for i, w := range want {
		if captured[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, captured[i], w)
		}
	}
}

func TestAddRemoteError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("remote already exists")
		},
	}
	if err := AddRemote(r, "origin", "https://github.com/x/y.git"); err == nil {
		t.Fatal("expected error from AddRemote")
	}
}
