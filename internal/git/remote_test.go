package git

import (
	"errors"
	"slices"
	"strings"
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

func TestRemotePruneNormal(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}
	if _, err := RemotePrune(r, "origin", false); err != nil {
		t.Fatalf("RemotePrune: %v", err)
	}
	want := []string{"remote", "prune", "origin"}
	if len(captured) != len(want) {
		t.Fatalf("args = %v, want %v", captured, want)
	}
	for i, w := range want {
		if captured[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, captured[i], w)
		}
	}
	if slices.Contains(captured, flagDryRun) {
		t.Error("--dry-run should not be present when dryRun=false")
	}
}

func TestRemotePruneDryRun(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}
	if _, err := RemotePrune(r, "origin", true); err != nil {
		t.Fatalf("RemotePrune dry-run: %v", err)
	}
	want := []string{"remote", "prune", "--dry-run", "origin"}
	if len(captured) != len(want) {
		t.Fatalf("args = %v, want %v", captured, want)
	}
	for i, w := range want {
		if captured[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, captured[i], w)
		}
	}
}

func TestRemotePruneParsesRefs(t *testing.T) {
	output := "Pruning origin\nURL: https://github.com/owner/repo.git\n * [pruned] origin/old\n * [pruned] origin/hotfix\n"
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return output, nil
		},
	}
	refs, err := RemotePrune(r, "origin", false)
	if err != nil {
		t.Fatalf("RemotePrune: %v", err)
	}
	want := []string{"origin/old", "origin/hotfix"}
	if len(refs) != len(want) {
		t.Fatalf("refs = %v, want %v", refs, want)
	}
	for i, w := range want {
		if refs[i] != w {
			t.Errorf("refs[%d] = %q, want %q", i, refs[i], w)
		}
	}
}

func TestRemotePruneDryRunParsesRefs(t *testing.T) {
	output := "Pruning origin\nURL: https://github.com/owner/repo.git\n * [would prune] origin/gone\n"
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return output, nil
		},
	}
	refs, err := RemotePrune(r, "origin", true)
	if err != nil {
		t.Fatalf("RemotePrune dry-run: %v", err)
	}
	if len(refs) != 1 || refs[0] != "origin/gone" {
		t.Errorf("refs = %v, want [origin/gone]", refs)
	}
}

func TestRemotePruneNoRefs(t *testing.T) {
	output := "Pruning origin\nURL: https://github.com/owner/repo.git\n"
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return output, nil
		},
	}
	refs, err := RemotePrune(r, "origin", false)
	if err != nil {
		t.Fatalf("RemotePrune: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected empty refs, got %v", refs)
	}
}

func TestRemotePruneErrorWrapping(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("connection refused")
		},
	}
	_, err := RemotePrune(r, "origin", false)
	if err == nil {
		t.Fatal("expected error from RemotePrune")
	}
	if !strings.Contains(err.Error(), "remote prune:") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "remote prune:")
	}
}
