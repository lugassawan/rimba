package git

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
)

// RemoteFailure records a prune error for a single remote.
type RemoteFailure struct {
	Remote string
	Err    error
}

// RemoteExists reports whether a remote with the given name is configured.
func RemoteExists(r Runner, name string) bool {
	_, err := r.Run("remote", "get-url", name)
	return err == nil
}

// RemotePrune runs `git remote prune <remote>`, deleting stale remote-tracking
// refs, and returns the names of the refs that were (or would be) pruned.
func RemotePrune(r Runner, remote string, dryRun bool) ([]string, error) {
	args := []string{"remote", "prune"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	args = append(args, remote)
	out, err := r.Run(args...)
	if err != nil {
		return nil, errhint.WithFix(
			fmt.Errorf("remote prune: %w", err),
			"check remote access, then run: git remote -v",
		)
	}
	return parsePrunedRefs(out), nil
}

// AddRemote adds a new remote with the given name and URL.
func AddRemote(r Runner, name, url string) error {
	_, err := r.Run("remote", "add", name, url)
	return err
}

// ListRemotes returns the names of all configured remotes by running `git remote`.
// It returns an empty (non-nil) slice when there are no remotes configured.
func ListRemotes(r Runner) ([]string, error) {
	out, err := r.Run("remote")
	if err != nil {
		return nil, errhint.WithFix(
			fmt.Errorf("list remotes: %w", err),
			"check the repository, then run: git remote -v",
		)
	}
	remotes := []string{}
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes, nil
}

// PruneRemotes calls RemotePrune for each remote in order. Pruned refs from all
// successful remotes are collected and returned. Any remote that fails is
// recorded in the failures slice; iteration continues regardless of errors.
// Both return values are initialized as non-nil empty slices.
func PruneRemotes(r Runner, remotes []string, dryRun bool) ([]string, []RemoteFailure) {
	pruned := []string{}
	failures := []RemoteFailure{}
	for _, remote := range remotes {
		refs, err := RemotePrune(r, remote, dryRun)
		if err != nil {
			failures = append(failures, RemoteFailure{Remote: remote, Err: err})
			continue
		}
		pruned = append(pruned, refs...)
	}
	return pruned, failures
}

// parsePrunedRefs extracts ref names from `git remote prune` output lines like
// ` * [pruned] origin/x` (live) and ` * [would prune] origin/x` (dry-run).
func parsePrunedRefs(out string) []string {
	refs := []string{}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "[pruned]") || strings.Contains(line, "[would prune]") {
			if fields := strings.Fields(line); len(fields) >= 2 {
				refs = append(refs, fields[len(fields)-1])
			}
		}
	}
	return refs
}
