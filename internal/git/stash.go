package git

import (
	"fmt"
	"strings"
)

// StashPushAndRef stashes all changes (including untracked files) with the given message
// and returns the stash object SHA so it can be applied or dropped by reference later.
func StashPushAndRef(r Runner, dir, message string) (string, error) {
	if _, err := r.RunInDir(dir, "stash", "push", "-u", "-m", message); err != nil {
		return "", fmt.Errorf("stash push: %w", err)
	}
	sha, err := r.RunInDir(dir, "rev-parse", "stash@{0}")
	if err != nil {
		return "", fmt.Errorf("resolve stash ref: %w", err)
	}
	return strings.TrimSpace(sha), nil
}

// StashApply applies the stash identified by sha in the given directory.
// git stash apply accepts commit SHAs directly.
// On conflict the error wraps the stderr output so conflict markers surface to the caller.
func StashApply(r Runner, dir, sha string) error {
	_, err := r.RunInDir(dir, "stash", "apply", sha)
	return err
}

// StashDrop drops the stash entry whose commit SHA matches sha.
// git stash drop requires stash@{N} form; this function resolves the SHA to the ref first.
func StashDrop(r Runner, dir, sha string) error {
	ref, err := stashRefBySHA(r, dir, sha)
	if err != nil {
		return err
	}
	_, err = r.RunInDir(dir, "stash", "drop", ref)
	return err
}

// stashRefBySHA walks the stash list to find the stash@{N} ref matching sha.
func stashRefBySHA(r Runner, dir, sha string) (string, error) {
	out, err := r.RunInDir(dir, "stash", "list", "--format=%H %gd")
	if err != nil {
		return "", fmt.Errorf("stash list: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(parts) == 2 && parts[0] == sha {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("stash entry %s not found in stash list", sha)
}
