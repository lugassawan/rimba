package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/lugassawan/rimba/internal/agentfile"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

// printAgentRefreshTips emits one tip per tier that has rimba-installed files on
// disk, suggesting the rimba init incantation to refresh them after a binary update,
// plus a distinct tip per tier that has a corrupt rimba block needing manual resolution.
// Silent when RIMBA_QUIET is set, no files are installed/corrupt, or a path is empty.
func printAgentRefreshTips(cmd *cobra.Command, home string, repoRoot string) {
	if _, ok := os.LookupEnv("RIMBA_QUIET"); ok {
		return
	}
	w := cmd.ErrOrStderr()
	if home != "" {
		printTierTips(w, agentfile.StatusGlobal(home), "at user level", "rimba init -g")
	}
	if repoRoot != "" {
		printTierTips(w, agentfile.StatusProject(repoRoot), "in this repo", "rimba init --agents")
	}
}

// printTierTips emits the refresh tip and/or corrupt tip for a single tier's statuses.
func printTierTips(w io.Writer, statuses []agentfile.FileStatus, tierLabel, refreshCmd string) {
	if anyInstalled(statuses) {
		fmt.Fprintf(w, "  Tip: agent files installed %s — run `%s` to refresh for this version.\n", tierLabel, refreshCmd)
	}
	if anyCorrupt(statuses) {
		fmt.Fprintf(w, "  Tip: an agent file %s has a corrupt rimba block — resolve manually.\n", tierLabel)
	}
}

func anyInstalled(statuses []agentfile.FileStatus) bool {
	for _, fs := range statuses {
		if fs.Installed {
			return true
		}
	}
	return false
}

func anyCorrupt(statuses []agentfile.FileStatus) bool {
	for _, fs := range statuses {
		if fs.Corrupt {
			return true
		}
	}
	return false
}

// resolvePostUpdateTipPaths returns (homeDir, repoRoot) for post-update tip emission.
// Returns empty strings for unresolvable tiers so callers skip the corresponding tip.
func resolvePostUpdateTipPaths() (string, string) {
	home, _ := os.UserHomeDir()
	var repoRoot string
	if root, err := git.RepoRoot(context.Background(), newRunner(context.Background())); err == nil { //nolint:contextcheck // no cobra cmd here — falls back to DefaultCommandTimeout
		repoRoot = root
	}
	return home, repoRoot
}
