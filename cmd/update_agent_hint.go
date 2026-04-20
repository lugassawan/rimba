package cmd

import (
	"fmt"
	"os"

	"github.com/lugassawan/rimba/internal/agentfile"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

// printAgentRefreshTips emits one tip per tier that has rimba-installed files on
// disk, suggesting the rimba init incantation to refresh them after a binary update.
// Silent when RIMBA_QUIET is set, no files are installed, or a path is empty.
func printAgentRefreshTips(cmd *cobra.Command, home string, repoRoot string) {
	if _, ok := os.LookupEnv("RIMBA_QUIET"); ok {
		return
	}
	w := cmd.ErrOrStderr()
	if home != "" && anyInstalled(agentfile.StatusGlobal(home)) {
		fmt.Fprintln(w, "  Tip: agent files installed at user level — run `rimba init -g` to refresh for this version.")
	}
	if repoRoot != "" && anyInstalled(agentfile.StatusProject(repoRoot)) {
		fmt.Fprintln(w, "  Tip: agent files installed in this repo — run `rimba init --agents` to refresh for this version.")
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

// resolvePostUpdateTipPaths returns (homeDir, repoRoot) for post-update tip emission.
// Returns empty strings for unresolvable tiers so callers skip the corresponding tip.
func resolvePostUpdateTipPaths() (string, string) {
	home, _ := os.UserHomeDir()
	var repoRoot string
	if root, err := git.RepoRoot(newRunner()); err == nil {
		repoRoot = root
	}
	return home, repoRoot
}
