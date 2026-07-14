package cmd

import (
	"fmt"
	"path/filepath"
	"time"

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
		"A lock proven to belong to a dead rimba sweep (marker + confirmed-dead owner PID) is " +
		"recovered automatically; everything else is report-only by default — use --fix to remove it.",
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

		if err := runLockCheck(cmd, commonDir); err != nil {
			return err
		}
		return runInterruptedCheck(cmd, r)
	},
}

// runLockCheck runs the stale index.lock scan/reap/report flow: sweep-manifest-confident
// removals first, then the age-based markerless flow (report-only, or --fix to remove).
func runLockCheck(cmd *cobra.Command, commonDir string) error {
	confidentRemovals := operations.ReapConfidentLocks(commonDir)
	reportConfidentReap(cmd, confidentRemovals)

	locks, err := operations.ScanWorktreeLocks(commonDir)
	if err != nil {
		return err
	}

	markerless, skippedAlive := partitionByAliveMarker(locks, operations.AliveSweepAdminDirs(commonDir))
	reportSkippedAliveMarker(cmd, skippedAlive)

	if len(markerless) == 0 {
		if len(confidentRemovals) == 0 && len(skippedAlive) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No stale index.lock files found.")
		}
		return nil
	}

	fix, _ := cmd.Flags().GetBool(flagFix)
	if !fix {
		return doctorReport(cmd, markerless)
	}
	return doctorFix(cmd, markerless)
}

// runInterruptedCheck scans for worktrees left in an "interrupted clean"
// state (a killed `git worktree remove` deleted every tracked file but never
// deregistered the worktree) and reports or removes them depending on --fix.
func runInterruptedCheck(cmd *cobra.Command, r git.Runner) error {
	interrupted, err := operations.ScanInterruptedWorktrees(cmd.Context(), r)
	if err != nil {
		return err
	}
	if len(interrupted) == 0 {
		return nil
	}

	fix, _ := cmd.Flags().GetBool(flagFix)
	if !fix {
		return reportInterrupted(cmd, interrupted)
	}
	return fixInterrupted(cmd, r, interrupted)
}

// reportInterrupted lists each interrupted worktree's path, branch, and
// deleted-file count, then hints at the fix commands.
func reportInterrupted(cmd *cobra.Command, worktrees []operations.InterruptedWorktree) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Interrupted worktree removals:")
	for _, w := range worktrees {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s [%s] (%d deleted file(s))\n", w.Path, w.Branch, w.DeletedCount)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nRun 'rimba remove <task> --force' to finish removing an affected worktree, "+
		"or 'rimba doctor --fix' to finish them all.")
	return nil
}

// fixInterrupted confirms (unless --force) and finishes removing each
// interrupted worktree. Only the worktree itself is removed — branch
// deletion stays out of scope for doctor (`rimba remove --force` does that).
func fixInterrupted(cmd *cobra.Command, r git.Runner, worktrees []operations.InterruptedWorktree) error {
	force, _ := cmd.Flags().GetBool(flagForce)
	if !force && !confirmRemoval(cmd, len(worktrees), "interrupted worktree(s)") {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	for _, w := range worktrees {
		if err := git.RemoveWorktree(cmd.Context(), r, w.Path, true); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to remove %s: %v\n", w.Path, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", w.Path)
	}
	return nil
}

func init() {
	doctorCmd.Flags().Bool(flagFix, false, "remove stale index.lock files")
	doctorCmd.Flags().Bool(flagForce, false, "skip confirmation when used with --fix")
	rootCmd.AddCommand(doctorCmd)
}

// partitionByAliveMarker splits locks into markerless and skippedAlive (a
// manifest claims it and the owner is confirmed alive — never touched).
func partitionByAliveMarker(locks []operations.LockInfo, aliveAdminDirs map[string]bool) (markerless, skippedAlive []operations.LockInfo) {
	for _, l := range locks {
		if aliveAdminDirs[filepath.Dir(l.Path)] {
			skippedAlive = append(skippedAlive, l)
			continue
		}
		markerless = append(markerless, l)
	}
	return markerless, skippedAlive
}

// reportConfidentReap announces locks recovered via sweep-manifest evidence
// before the manual, age-based flow below even runs.
func reportConfidentReap(cmd *cobra.Command, removals []operations.LockRemoval) {
	if len(removals) == 0 {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Recovered %d stale index.lock file(s) left by an interrupted sweep:\n", len(removals))
	for _, rm := range removals {
		if rm.Err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s (failed to remove: %v)\n", rm.Path, rm.Err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", rm.Path)
	}
}

// reportSkippedAliveMarker announces locks left untouched because a sweep
// manifest proves a still-running process owns them.
func reportSkippedAliveMarker(cmd *cobra.Command, locks []operations.LockInfo) {
	for _, l := range locks {
		fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s: owned by a sweep that is still running.\n", l.Path)
	}
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

	removable, skipped := partitionByAge(locks, operations.MinLockAge)
	for _, l := range skipped {
		fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s: too recent (age %s) to safely assume the owning process has exited.\n", l.Path, resolver.FormatAge(l.ModTime))
	}
	if len(removable) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No locks old enough to remove.")
		return nil
	}

	force, _ := cmd.Flags().GetBool(flagForce)
	if !force && !confirmRemoval(cmd, len(removable), "stale index.lock file(s)") {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
		return nil
	}

	for _, rm := range operations.RemoveStaleLocks(removable) {
		if rm.Err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Failed to remove %s: %v\n", rm.Path, rm.Err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed %s\n", rm.Path)
	}
	return nil
}

// partitionByAge splits locks into those old enough for --fix to remove and
// those too young to safely assume abandoned.
func partitionByAge(locks []operations.LockInfo, minAge time.Duration) (removable, skipped []operations.LockInfo) {
	for _, l := range locks {
		if l.Age < minAge {
			skipped = append(skipped, l)
			continue
		}
		removable = append(removable, l)
	}
	return removable, skipped
}
