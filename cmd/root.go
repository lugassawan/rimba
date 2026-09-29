package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	rimbadebug "github.com/lugassawan/rimba/internal/debug"
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

	// annotationSkipConfig marks commands that must run without a resolved
	// rimba config (e.g. status/clean/log outside a configured repo).
	annotationSkipConfig = "skipConfig"
	annotationValueTrue  = "true"

	cmdNameStatus = "status"

	exitCodePanic = 2 // Go runtime panic exit code
)

// commandName stores the resolved command name for JSON error reporting.
var commandName string

// lastRecorder captures PersistentPreRunE's Recorder for the most recently
// invoked command, since cobra never copies a subcommand's context back up
// to rootCmd (so Execute() can't recover it via rootCmd.Context()). Reset to
// nil at the top of every PersistentPreRunE to avoid leaking a stale value.
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
			_ = os.Setenv(rimbadebug.EnvVar, "1")
		}

		// Skip config for Cobra internals (completion, __complete)
		if cmd.Name() == "completion" || cmd.Name() == "__complete" {
			return nil
		}

		// Skip config if any command in the chain is annotated
		for c := cmd; c != nil; c = c.Parent() {
			if c.Annotations != nil && c.Annotations[annotationSkipConfig] == annotationValueTrue {
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

		rec := observability.Maybe(true, openObservabilitySink(cfg, repoRoot), commandName, "", "", version)
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

func Execute() (err error) {
	updater.SweepOldBinary()
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	defer func() { lastRecorder = nil }() // runs last, even when finalizeRecorder re-panics
	// lastRecorder is set inside ExecuteContext (see its doc comment), so it is
	// read when the defer runs. recover() must be called directly in the literal.
	done := false // stays false on panic or runtime.Goexit
	defer func() { finalizeRecorder(lastRecorder, recover(), done, err) }()

	err = rootCmd.ExecuteContext(ctx)
	done = true
	return err
}

// finalizeRecorder finalizes and closes rec, then re-raises a recovered panic.
// rec may be nil: Finalize and Close are nil-safe, so the panic is always re-raised.
func finalizeRecorder(rec *observability.Recorder, p any, done bool, err error) {
	outcome, exitCode := observability.OutcomeSuccess, exitCodeFor(err)
	switch {
	case p != nil:
		outcome, exitCode, err = observability.OutcomeError, exitCodePanic, observability.PanicError(p)
	case !done:
		outcome, exitCode, err = observability.OutcomeError, 1, observability.ErrIncomplete
	case err != nil:
		outcome = observability.OutcomeError
	}
	rec.Finalize(outcome, exitCode, err)
	_ = rec.Close()
	if p != nil {
		panic(p)
	}
}

func exitCodeFor(err error) int {
	if silent, ok := errors.AsType[*output.SilentError](err); ok {
		return silent.ExitCode
	}
	if err != nil {
		return 1
	}
	return 0
}
