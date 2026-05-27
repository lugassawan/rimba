package git

import (
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
)

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
