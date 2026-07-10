package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

const flagFix = "fix"

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose stale git index.lock files left by killed worktree operations",
	Long: "Scans every linked worktree's admin directory for a stale index.lock file — " +
		"the kind of leftover a killed `git worktree remove` on a very large tree can leave behind. " +
		"Report-only by default; use --fix to remove them.",
	Example: `  rimba doctor
  rimba doctor --fix
  rimba doctor --fix --force`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner(cmd.Context())
		commonDir, err := git.CommonDir(cmd.Context(), r)
		if err != nil {
			return err
		}

		locks, err := operations.ScanWorktreeLocks(commonDir)
		if err != nil {
			return err
		}

		if len(locks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No stale index.lock files found.")
			return nil
		}

		fix, _ := cmd.Flags().GetBool(flagFix)
		if !fix {
			return doctorReport(cmd, locks)
		}
		return doctorFix(cmd, locks)
	},
}

func init() {
	doctorCmd.Flags().Bool(flagFix, false, "remove stale index.lock files")
	doctorCmd.Flags().Bool(flagForce, false, "skip confirmation when used with --fix")
	rootCmd.AddCommand(doctorCmd)
}

// doctorReport lists each stale lock's path and age.
func doctorReport(cmd *cobra.Command, locks []operations.LockInfo) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Stale index.lock files:")
	for _, l := range locks {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (age: %s)\n", l.Path, resolver.FormatAge(l.ModTime))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nRun 'rimba doctor --fix' to remove them.")
	return nil
}

// doctorFix confirms (unless --force) and removes each stale lock, reporting
// per-lock outcomes. A lock can legitimately belong to an in-flight git
// process, so it always warns before touching anything.
func doctorFix(cmd *cobra.Command, locks []operations.LockInfo) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Warning: a lock may belong to a running git process; make sure no git command is in flight.")

	force, _ := cmd.Flags().GetBool(flagForce)
	if !force && !confirmRemoval(cmd, len(locks), "stale index.lock file(s)") {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	for _, rm := range operations.RemoveStaleLocks(locks) {
		if rm.Err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to remove %s: %v\n", rm.Path, rm.Err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", rm.Path)
	}
	return nil
}
