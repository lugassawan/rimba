package cmd

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

const (
	errWorktreeNotFound = "worktree not found for task %q"
	flagForce           = "force"
	flagJSON            = "json"
	flagNoColor         = "no-color"
	flagSkipDeps        = "skip-deps"
	flagSkipHooks       = "skip-hooks"
	flagStaleDays       = "stale-days"
	defaultStaleDays    = 14

	hintSkipDeps  = "Skip dependency installation (faster, but requires manual install)"
	hintSkipHooks = "Skip post-create hooks (faster, but automation won't run)"
)

// commandName stores the resolved command name for JSON error reporting.
var commandName string

var rootCmd = &cobra.Command{
	Use:           "rimba",
	Short:         "Git worktree lifecycle manager",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		commandName = strings.TrimPrefix(cmd.CommandPath(), "rimba ")

		// Skip config for Cobra internals (completion, __complete)
		if cmd.Name() == "completion" || cmd.Name() == "__complete" {
			return nil
		}

		// Skip config if any command in the chain is annotated
		for c := cmd; c != nil; c = c.Parent() {
			if c.Annotations != nil && c.Annotations["skipConfig"] == "true" {
				return nil
			}
		}

		r := newRunner()
		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		cfg, err := config.Resolve(repoRoot)
		if err != nil {
			return err
		}

		// Auto-derive missing fields
		repoName := filepath.Base(repoRoot)
		var defaultBranch string
		if cfg.DefaultSource == "" {
			defaultBranch, err = git.DefaultBranch(r)
			if err != nil {
				return err
			}
		}
		cfg.FillDefaults(repoName, defaultBranch)

		cmd.SetContext(config.WithConfig(cmd.Context(), cfg))
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().Bool(flagJSON, false, "output in JSON format")
	rootCmd.PersistentFlags().Bool(flagNoColor, false, "disable colored output")

	originalHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		var hint <-chan *updater.CheckResult
		if cmd == rootCmd {
			hint = checkUpdateHint(version, 2*time.Second)
			printBanner(cmd)
		}
		originalHelp(cmd, args)
		if hint != nil {
			if result := collectHint(hint); result != nil {
				printUpdateHint(cmd, result)
			}
		}
	})
}

// IsJSONMode returns true if the --json flag was set on the root command.
func IsJSONMode() bool {
	v, _ := rootCmd.PersistentFlags().GetBool(flagJSON)
	return v
}

// CommandName returns the resolved command name from the last execution.
func CommandName() string {
	return commandName
}

func Execute() error {
	return rootCmd.Execute()
}
