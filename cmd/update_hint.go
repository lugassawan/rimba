package cmd

import (
	"context"
	"fmt"
	"time"

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
	ch := make(chan *updater.CheckResult, 1)

	if updater.IsDevVersion(version) {
		close(ch)
		return ch
	}

	// Derive a timeout-scoped child so the HTTP request is actually cancelled
	// when timeout elapses, not just when the caller stops waiting.
	tctx, cancel := context.WithTimeout(ctx, timeout)

	// Read newUpdater before spawning the goroutine: goroutine launch
	// establishes a happens-before edge, preventing a data race with
	// test overrides that restore the variable via t.Cleanup.
	u := newUpdater(version)
	go func() {
		defer cancel()
		result, err := u.Check(tctx)
		if err != nil || result.UpToDate {
			close(ch)
			return
		}
		ch <- result
	}()

	// Return a channel that resolves with the result or nil when tctx expires.
	out := make(chan *updater.CheckResult, 1)
	go func() {
		defer cancel()
		select {
		case r, ok := <-ch:
			if ok {
				out <- r
			} else {
				close(out)
			}
		case <-tctx.Done():
			close(out)
		}
	}()

	return out
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
