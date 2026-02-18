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
	Short:       "Manage Git hooks for automatic worktree cleanup",
	Long:        "Install or remove a post-merge Git hook that automatically cleans merged worktrees after git pull.",
	Annotations: map[string]string{"skipConfig": "true"},
}

var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the post-merge hook for automatic cleanup",
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

		err = hook.Install(hooksDir, branch)
		if errors.Is(err, hook.ErrAlreadyInstalled) {
			fmt.Fprintln(cmd.OutOrStdout(), "Rimba post-merge hook is already installed.")
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Installed post-merge hook (branch: %s)\n", branch)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", hook.Check(hooksDir).HookPath)
		return nil
	},
}

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the post-merge hook",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		hooksDir, err := git.HooksDir(r)
		if err != nil {
			return err
		}

		err = hook.Uninstall(hooksDir)
		if errors.Is(err, hook.ErrNotInstalled) {
			return errors.New("rimba post-merge hook is not installed")
		}
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Uninstalled rimba post-merge hook.")
		return nil
	},
}

var hookStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show post-merge hook status",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		hooksDir, err := git.HooksDir(r)
		if err != nil {
			return err
		}

		s := hook.Check(hooksDir)
		if s.Installed {
			fmt.Fprintln(cmd.OutOrStdout(), "Rimba post-merge hook is installed.")
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", s.HookPath)
			if s.HasOther {
				fmt.Fprintln(cmd.OutOrStdout(), "  (hook file also contains other content)")
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Rimba post-merge hook is not installed.")
		}
		return nil
	},
}
