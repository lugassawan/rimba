package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

const (
	flagDebug        = "debug"
	flagDetail       = "detail"
	flagDryRun       = "dry-run"
	flagForce        = "force"
	flagJSON         = "json"
	flagNoColor      = "no-color"
	flagPush         = "push"
	flagSkipDeps     = "skip-deps"
	flagSkipHooks    = "skip-hooks"
	flagStaleDays    = "stale-days"
	defaultStaleDays = 14

	hintDryRun          = "Preview what would be done without making changes"
	hintSkipDeps        = "Skip dependency installation (faster, but requires manual install)"
	hintSkipHooks       = "Skip post-create hooks (faster, but automation won't run)"
	hintSkipHooksRename = "Skip post-rename hooks (faster, but automation won't run)"
)

// commandName stores the resolved command name for JSON error reporting.
var commandName string

// lastRecorder captures the Recorder built inside PersistentPreRunE for the
// most recently invoked command. Execute() cannot recover it via
// rootCmd.Context() after ExecuteContext returns: cobra only copies context
// from a parent command down to a child whose own ctx is still nil (see
// Command.ExecuteC's `if cmd.ctx == nil { cmd.ctx = c.ctx }`) — it never
// copies back up. cmd.SetContext inside PersistentPreRunE is called on the
// *invoked* (sub)command object, not on rootCmd, so rootCmd.ctx is left
// exactly as ExecuteContext set it and never reflects the config/Recorder
// context built during preRun (verified empirically: a minimal cobra repro
// showed root.Context() nil while the invoked subcommand's Context() carried
// the value). Reset to nil at the top of every PersistentPreRunE invocation
// so a command that skips the observability build (completion, __complete,
// skipConfig-annotated chains) never leaks a previous invocation's Recorder.
var lastRecorder *observability.Recorder

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
		lastRecorder = nil

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

		r := newRunner(cmd.Context())
		repoRoot, err := git.MainRepoRoot(cmd.Context(), r)
		if err != nil {
			return err
		}

		cfg, err := config.Resolve(repoRoot)
		if err != nil {
			return err
		}

		// Auto-derive missing fields.
		repoName := filepath.Base(repoRoot)
		defaultBranch, err := git.DefaultBranch(cmd.Context(), r)
		if err != nil {
			return err
		}
		cfg.FillDefaults(repoName, defaultBranch)

		if err := cfg.Validate(); err != nil {
			return err
		}

		// Only open the sink (and thus create today's day-files) when
		// observability is actually enabled — RIMBA_NO_OBSERVABILITY / a
		// config [observability] enabled=false must leave zero filesystem
		// footprint, not merely an empty file with no records written to it.
		var rec *observability.Recorder
		if cfg.IsObservabilityEnabled() {
			sink, sinkErr := observability.NewFileSink(repoRoot, cfg.ObservabilityRetentionDays())
			if sinkErr == nil {
				rec = observability.Maybe(true, sink, commandName, "", "", version)
			} else if os.Getenv("RIMBA_DEBUG") != "" {
				// sinkErr is swallowed here deliberately: observability must never
				// block a command from running (e.g. a read-only cache dir).
				// Surface it only under RIMBA_DEBUG, to stderr, the same channel
				// debug output already uses.
				fmt.Fprintf(os.Stderr, "\n[debug] observability disabled: %v\n", sinkErr)
			}
		}
		lastRecorder = rec

		ctx := observability.WithRecorder(cmd.Context(), rec)
		ctx = config.WithConfig(ctx, cfg)
		cmd.SetContext(ctx)
		return nil
	},
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().Bool(flagJSON, false, "output in JSON format")
	rootCmd.PersistentFlags().Bool(flagNoColor, false, "disable colored output")
	rootCmd.PersistentFlags().Bool(flagDebug, false, "log git commands and timings to stderr")
	rootCmd.PersistentFlags().Bool(flagYes, false, hintYes)

	originalHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		var hint <-chan *updater.CheckResult
		if cmd == rootCmd {
			ctx := cmd.Context()
			if ctx == nil {
				// cmd.Context() is nil when HelpFunc is called outside of
				// ExecuteContext (e.g., directly in tests).
				ctx = context.Background()
			}
			hint = checkUpdateHint(ctx, version, 2*time.Second)
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
	updater.SweepOldBinary()
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := rootCmd.ExecuteContext(ctx)

	// See lastRecorder's doc comment: rootCmd.Context() does not reflect the
	// context PersistentPreRunE set on the invoked (sub)command, so the
	// Recorder built there is captured via lastRecorder instead.
	if rec := lastRecorder; rec != nil {
		defer rec.Close() // registered first so it runs LAST (after Finalize below)
		exitCode := 0
		var silent *output.SilentError
		if errors.As(err, &silent) {
			exitCode = silent.ExitCode
		} else if err != nil {
			exitCode = 1
		}
		outcome := observability.OutcomeSuccess
		if err != nil {
			outcome = observability.OutcomeError
		}
		rec.Finalize(outcome, exitCode, err)
	}

	return err
}
