package cmd

import (
	"errors"
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hook"
	"github.com/spf13/cobra"
)

func init() {
	hookCmd.AddCommand(hookInstallCmd)
	hookCmd.AddCommand(hookUninstallCmd)
	hookCmd.AddCommand(hookStatusCmd)
	rootCmd.AddCommand(hookCmd)
}

var hookCmd = &cobra.Command{
	Use:         "hook",
	Short:       "Manage Git hooks for worktree workflow",
	Long:        "Install or remove Git hooks: a post-merge hook for automatic cleanup and a pre-commit hook that prevents direct commits to main/master.",
	Annotations: map[string]string{"skipConfig": "true"},
}

var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install post-merge and pre-commit hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		branch, err := resolveMainBranch(r)
		if err != nil {
			return fmt.Errorf("resolve main branch: %w", err)
		}

		hooksDir, err := git.HooksDir(r)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()

		// Install post-merge hook
		err = hook.Install(hooksDir, hook.PostMergeHook, hook.PostMergeBlock(branch))
		if errors.Is(err, hook.ErrAlreadyInstalled) {
			fmt.Fprintln(out, "Rimba post-merge hook is already installed.")
		} else if err != nil {
			return fmt.Errorf("install post-merge hook: %w", err)
		} else {
			fmt.Fprintf(out, "Installed post-merge hook (branch: %s)\n", branch)
		}

		// Install pre-commit hook
		err = hook.Install(hooksDir, hook.PreCommitHook, hook.PreCommitBlock())
		if errors.Is(err, hook.ErrAlreadyInstalled) {
			fmt.Fprintln(out, "Rimba pre-commit hook is already installed.")
		} else if err != nil {
			return fmt.Errorf("install pre-commit hook: %w", err)
		} else {
			fmt.Fprintln(out, "Installed pre-commit hook (protects main/master)")
		}

		return nil
	},
}

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove post-merge and pre-commit hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		hooksDir, err := git.HooksDir(r)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		var uninstalled int

		// Uninstall post-merge hook
		err = hook.Uninstall(hooksDir, hook.PostMergeHook)
		if errors.Is(err, hook.ErrNotInstalled) {
			// skip silently
		} else if err != nil {
			return fmt.Errorf("uninstall post-merge hook: %w", err)
		} else {
			fmt.Fprintln(out, "Uninstalled rimba post-merge hook.")
			uninstalled++
		}

		// Uninstall pre-commit hook
		err = hook.Uninstall(hooksDir, hook.PreCommitHook)
		if errors.Is(err, hook.ErrNotInstalled) {
			// skip silently
		} else if err != nil {
			return fmt.Errorf("uninstall pre-commit hook: %w", err)
		} else {
			fmt.Fprintln(out, "Uninstalled rimba pre-commit hook.")
			uninstalled++
		}

		if uninstalled == 0 {
			return errors.New("rimba hooks are not installed")
		}
		return nil
	},
}

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show hook status",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		hooksDir, err := git.HooksDir(r)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()

		// Post-merge status
		pm := hook.Check(hooksDir, hook.PostMergeHook)
		if pm.Installed {
			fmt.Fprintln(out, "Rimba post-merge hook is installed.")
			fmt.Fprintf(out, "  %s\n", pm.HookPath)
			if pm.HasOther {
				fmt.Fprintln(out, "  (hook file also contains other content)")
			}
		} else {
			fmt.Fprintln(out, "Rimba post-merge hook is not installed.")
		}

		// Pre-commit status
		pc := hook.Check(hooksDir, hook.PreCommitHook)
		if pc.Installed {
			fmt.Fprintln(out, "Rimba pre-commit hook is installed.")
			fmt.Fprintf(out, "  %s\n", pc.HookPath)
			if pc.HasOther {
				fmt.Fprintln(out, "  (hook file also contains other content)")
			}
		} else {
			fmt.Fprintln(out, "Rimba pre-commit hook is not installed.")
		}

		return nil
	},
}
