package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/spf13/cobra"
)

const (
	flagWith  = "with"
	flagIDE   = "ide"
	flagAgent = "agent"
)

func init() {
	openCmd.Flags().StringP(flagWith, "w", "", "run a named shortcut from [open] config")
	openCmd.Flags().Bool(flagIDE, false, "run the 'ide' shortcut from [open] config")
	openCmd.Flags().Bool(flagAgent, false, "run the 'agent' shortcut from [open] config")
	openCmd.MarkFlagsMutuallyExclusive(flagWith, flagIDE, flagAgent)

	_ = openCmd.RegisterFlagCompletionFunc(flagWith, func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeOpenShortcuts(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(openCmd)
}

var openCmd = &cobra.Command{
	Use:   "open <task> [command args...]",
	Short: "Open a worktree or run a command inside it",
	Long: `Prints the worktree path for the given task, or executes a command inside it.

Examples:
  rimba open my-task              # Print worktree path
  cd $(rimba open my-task)        # Navigate to worktree
  rimba open my-task --ide        # Run the 'ide' shortcut
  rimba open my-task --agent      # Run the 'agent' shortcut
  rimba open my-task -w test      # Run a named shortcut
  rimba open my-task npm start    # Run an inline command

Shortcuts are configured in .rimba/settings.toml:
  [open]
  ide = "code ."
  agent = "claude"
  test = "npm test"`,
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

		cmdArgs, err := resolveOpenCommand(cmd, args[1:])
		if err != nil {
			return err
		}

		if cmdArgs == nil {
			fmt.Fprint(cmd.OutOrStdout(), wt.Path)
			return nil
		}

		sub := exec.Command(cmdArgs[0], cmdArgs[1:]...) //nolint:gosec // Intentional: user specifies the command to run
		sub.Dir = wt.Path
		sub.Stdin = os.Stdin
		sub.Stdout = os.Stdout
		sub.Stderr = os.Stderr

		if err := sub.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("failed to run %q: %w", cmdArgs[0], err)
		}

		return nil
	},
}

// resolveOpenCommand determines the command to execute based on flags and inline args.
// Returns nil when no command should be run (print path mode).
func resolveOpenCommand(cmd *cobra.Command, inlineArgs []string) ([]string, error) {
	withName, _ := cmd.Flags().GetString(flagWith)
	useIDE, _ := cmd.Flags().GetBool(flagIDE)
	useAgent, _ := cmd.Flags().GetBool(flagAgent)

	var shortcutName string
	switch {
	case useIDE:
		shortcutName = flagIDE
	case useAgent:
		shortcutName = flagAgent
	case withName != "":
		shortcutName = withName
	}

	if shortcutName != "" {
		if len(inlineArgs) > 0 {
			return nil, fmt.Errorf("cannot combine --%s with inline command arguments", flagForShortcut(useIDE, useAgent))
		}
		return resolveShortcut(cmd, shortcutName)
	}

	if len(inlineArgs) > 0 {
		return inlineArgs, nil
	}
	return nil, nil
}

// resolveShortcut looks up a shortcut name in the [open] config section.
func resolveShortcut(cmd *cobra.Command, name string) ([]string, error) {
	cfg := config.FromContext(cmd.Context())
	if cfg == nil || cfg.Open == nil {
		return nil, fmt.Errorf("no [open] section in config; add shortcuts to .rimba/settings.toml:\n  [open]\n  %s = \"your-command\"", name)
	}

	value, ok := cfg.Open[name]
	if !ok {
		return nil, fmt.Errorf("shortcut %q not found in [open] config; available: %s", name, availableShortcuts(cfg.Open))
	}

	parts := strings.Fields(value)
	if len(parts) == 0 {
		return nil, fmt.Errorf("shortcut %q has empty value in [open] config", name)
	}
	return parts, nil
}

// flagForShortcut returns the flag name used for error messages.
func flagForShortcut(useIDE, useAgent bool) string {
	if useIDE {
		return flagIDE
	}
	if useAgent {
		return flagAgent
	}
	return flagWith
}

// availableShortcuts returns a sorted, comma-separated list of shortcut names.
func availableShortcuts(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
