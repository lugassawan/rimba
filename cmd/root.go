package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

const (
	flagDebug        = "debug"
	flagDetail       = "detail"
	flagForce        = "force"
	flagJSON         = "json"
	flagNoColor      = "no-color"
	flagSkipDeps     = "skip-deps"
	flagSkipHooks    = "skip-hooks"
	flagStaleDays    = "stale-days"
	defaultStaleDays = 14

	hintSkipDeps  = "Skip dependency installation (faster, but requires manual install)"
	hintSkipHooks = "Skip post-create hooks (faster, but automation won't run)"
)

// commandName stores the resolved command name for JSON error reporting.
var commandName string

var rootCmd = &cobra.Command{
	Use:   "rimba",
	Short: "Manage git worktrees — create, sync, merge, and organize branches",
	Long: `rimba is a git worktree manager. It creates, syncs, merges, and organizes
branches as isolated worktrees so you can work on multiple tasks in parallel.

Persistent flags (available on every command):
  --json      Output in JSON format (where supported)
  --no-color  Disable colored output (also respects NO_COLOR)
  --debug     Log git commands and timings to stderr (also respects RIMBA_DEBUG=1)`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		commandName = strings.TrimPrefix(cmd.CommandPath(), "rimba ")

		if debug, _ := cmd.Flags().GetBool(flagDebug); debug {
			_ = os.Setenv("RIMBA_DEBUG", "1")
		}

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

		if err := cfg.Validate(); err != nil {
			return err
		}

		cmd.SetContext(config.WithConfig(cmd.Context(), cfg))
		return nil
	},
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().Bool(flagJSON, false, "output in JSON format")
	rootCmd.PersistentFlags().Bool(flagNoColor, false, "disable colored output")
	rootCmd.PersistentFlags().Bool(flagDebug, false, "Log git commands and timings to stderr")

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
