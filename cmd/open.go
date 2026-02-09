package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(openCmd)
}

var openCmd = &cobra.Command{
	Use:   "open <task> [command args...]",
	Short: "Open a worktree or run a command inside it",
	Long: `Prints the worktree path for the given task, or executes a command inside it.

Examples:
  rimba open my-task              # Print worktree path
  cd $(rimba open my-task)        # Navigate to worktree
  rimba open my-task code .       # Open in VS Code
  rimba open my-task claude       # Launch claude in worktree`,
	Args: cobra.MinimumNArgs(1),
	FParseErrWhitelist: cobra.FParseErrWhitelist{
		UnknownFlags: true,
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		r := newRunner()

		wt, err := findWorktree(r, task)
		if err != nil {
			return err
		}

		if len(args) == 1 {
			fmt.Fprint(cmd.OutOrStdout(), wt.Path)
			return nil
		}

		sub := exec.Command(args[1], args[2:]...) //nolint:gosec // Intentional: user specifies the command to run
		sub.Dir = wt.Path
		sub.Stdin = os.Stdin
		sub.Stdout = os.Stdout
		sub.Stderr = os.Stderr

		if err := sub.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("failed to run %q: %w", args[1], err)
		}

		return nil
	},
}
