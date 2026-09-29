package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	rimbadebug "github.com/lugassawan/rimba/internal/debug"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

// newUpdater is a package-level function variable for creating an Updater.
// Tests can override this to inject a mock server.
var newUpdater func(string) *updater.Updater = updater.New

// checkUpdateHint runs a background version check bounded by timeout.
// Returns nil if the version is dev, the check fails, times out, or is up to date.
// The check is cancelled when ctx is done or timeout elapses — whichever comes first.
func checkUpdateHint(ctx context.Context, version string, timeout time.Duration) <-chan *updater.CheckResult {
	out := make(chan *updater.CheckResult, 1)

	if updater.IsDevVersion(version) {
		close(out)
		return out
	}

	// Bound the check by timeout so the HTTP request is actually cancelled, not
	// just abandoned. u.Check honours tctx, so the goroutine always terminates
	// within timeout — no leak, and the result can never be lost to a racing
	// cancellation (there is no select between "result" and "ctx done").
	tctx, cancel := context.WithTimeout(ctx, timeout)

	// Read newUpdater before spawning the goroutine: goroutine launch
	// establishes a happens-before edge, preventing a data race with
	// test overrides that restore the variable via t.Cleanup.
	u := newUpdater(version)
	go func() {
		defer cancel()
		defer func() { closeOnHintPanic(out, recover()) }()
		result, err := u.Check(tctx)
		if err != nil || result.UpToDate {
			close(out)
			return
		}
		out <- result // buffered (cap 1) — never blocks
	}()

	return out
}

// closeOnHintPanic turns a panic in the best-effort update check into "no hint"
// (a failing check must not crash the CLI); the cause shows under RIMBA_DEBUG.
func closeOnHintPanic(out chan<- *updater.CheckResult, p any) {
	if p == nil {
		return
	}
	if os.Getenv(rimbadebug.EnvVar) != "" {
		parallel.ReportPanic("[debug] update check panicked (ignored)", p, debug.Stack())
	}
	close(out)
}

// collectHint reads the result from the hint channel. Returns nil if the
// channel was closed (no update available, timed out, or error).
func collectHint(ch <-chan *updater.CheckResult) *updater.CheckResult {
	r, ok := <-ch
	if !ok {
		return nil
	}
	return r
}

// printUpdateHint prints a yellow-colored update notification.
func printUpdateHint(cmd *cobra.Command, result *updater.CheckResult) {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	msg := fmt.Sprintf(
		"Update available: %s → %s — run \"rimba update\" to upgrade",
		result.CurrentVersion, result.LatestVersion,
	)
	fmt.Fprintln(cmd.OutOrStdout(), p.Paint(msg, termcolor.Yellow))
}
