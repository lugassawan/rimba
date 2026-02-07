package cmd

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long:  "Lists all git worktrees with their branch, path, and status (dirty, ahead/behind).",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		if cfg == nil {
			return fmt.Errorf(errNoConfig)
		}

		r := &git.ExecRunner{}

		repoRoot, err := git.RepoRoot(r)
		if err != nil {
			return err
		}

		entries, err := git.ListWorktrees(r)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TASK\tBRANCH\tPATH\tSTATUS")

		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)

		for _, e := range entries {
			if e.Bare {
				continue
			}

			task := resolver.TaskFromBranch(e.Branch, cfg.DefaultPrefix)

			// Determine relative path for display
			displayPath := e.Path
			if rel, err := filepath.Rel(wtDir, e.Path); err == nil && len(rel) < len(displayPath) {
				displayPath = rel
			}

			// Check status
			status := ""
			if dirty, err := git.IsDirty(r, e.Path); err == nil && dirty {
				status = "[dirty]"
			}

			ahead, behind, _ := git.AheadBehind(r, e.Path)
			if ahead > 0 || behind > 0 {
				ab := fmt.Sprintf("[+%d/-%d]", ahead, behind)
				if status != "" {
					status += " " + ab
				} else {
					status = ab
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", task, e.Branch, displayPath, status)
		}

		return w.Flush()
	},
}
