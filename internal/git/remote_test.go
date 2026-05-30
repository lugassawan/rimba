package git

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestDeleteRemoteBranchSuccess(t *testing.T) {
	var captured []string
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			captured = args
			return "", nil
		},
	}
	if err := DeleteRemoteBranch(r, "origin", "feature/done"); err != nil {
		t.Fatalf("DeleteRemoteBranch: %v", err)
	}
	want := []string{"push", "origin", "--delete", "feature/done"}
	if len(captured) != len(want) {
		t.Fatalf("args = %v, want %v", captured, want)
	}
	for i, w := range want {
		if captured[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, captured[i], w)
		}
	}
}

func TestDeleteRemoteBranchRefGoneIsIdempotent(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("error: remote ref does not exist")
		},
	}
	if err := DeleteRemoteBranch(r, "origin", "gone-branch"); err != nil {
		t.Fatalf("expected nil for already-gone remote ref, got: %v", err)
	}
}

func TestDeleteRemoteBranchFailure(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("connection refused")
		},
	}
	err := DeleteRemoteBranch(r, "origin", "feature/x")
	if err == nil {
		t.Fatal("expected non-nil error for remote failure")
	}
	if !strings.Contains(err.Error(), "delete remote branch") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "delete remote branch")
	}
	if !strings.Contains(err.Error(), "To fix:") {
		t.Errorf("error = %q, want it to contain hint", err.Error())
	}
}

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

func TestListRemotesTwoRemotes(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "origin\nupstream\n", nil
		},
	}
	remotes, err := ListRemotes(r)
	if err != nil {
		t.Fatalf("ListRemotes: %v", err)
	}
	want := []string{"origin", "upstream"}
	if len(remotes) != len(want) {
		t.Fatalf("remotes = %v, want %v", remotes, want)
	}
	for i, w := range want {
		if remotes[i] != w {
			t.Errorf("remotes[%d] = %q, want %q", i, remotes[i], w)
		}
	}
}

func TestListRemotesEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", nil
		},
	}
	remotes, err := ListRemotes(r)
	if err != nil {
		t.Fatalf("ListRemotes: %v", err)
	}
	if remotes == nil {
		t.Fatal("expected non-nil slice, got nil")
	}
	if len(remotes) != 0 {
		t.Errorf("expected empty slice, got %v", remotes)
	}
}

func TestListRemotesRunnerError(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			return "", errors.New("not a git repository")
		},
	}
	_, err := ListRemotes(r)
	if err == nil {
		t.Fatal("expected error from ListRemotes")
	}
	if !strings.Contains(err.Error(), "list remotes:") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "list remotes:")
	}
}

func TestPruneRemotesBothSucceed(t *testing.T) {
	calls := map[string]string{
		"origin":   " * [pruned] origin/old\n",
		"upstream": " * [pruned] upstream/gone\n",
	}
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			remote := args[len(args)-1]
			return calls[remote], nil
		},
	}
	pruned, failures := PruneRemotes(r, []string{"origin", "upstream"}, false)
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
	want := []string{"origin/old", "upstream/gone"}
	if len(pruned) != len(want) {
		t.Fatalf("pruned = %v, want %v", pruned, want)
	}
	for i, w := range want {
		if pruned[i] != w {
			t.Errorf("pruned[%d] = %q, want %q", i, pruned[i], w)
		}
	}
}

func TestPruneRemotesPartialFailure(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			remote := args[len(args)-1]
			if remote == "upstream" {
				return "", errors.New("could not read from remote")
			}
			return " * [pruned] origin/old\n", nil
		},
	}
	pruned, failures := PruneRemotes(r, []string{"origin", "upstream"}, false)
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure, got %d: %v", len(failures), failures)
	}
	if failures[0].Remote != "upstream" {
		t.Errorf("failures[0].Remote = %q, want %q", failures[0].Remote, "upstream")
	}
	if failures[0].Err == nil {
		t.Error("failures[0].Err should not be nil")
	}
	if len(pruned) != 1 || pruned[0] != "origin/old" {
		t.Errorf("pruned = %v, want [origin/old]", pruned)
	}
}

func TestPruneRemotesEmptyInput(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			t.Error("runner should not be called for empty input")
			return "", nil
		},
	}
	pruned, failures := PruneRemotes(r, []string{}, false)
	if pruned == nil {
		t.Error("expected non-nil pruned slice")
	}
	if failures == nil {
		t.Error("expected non-nil failures slice")
	}
	if len(pruned) != 0 {
		t.Errorf("expected empty pruned, got %v", pruned)
	}
	if len(failures) != 0 {
		t.Errorf("expected empty failures, got %v", failures)
	}
}
