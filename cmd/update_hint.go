package cmd

import (
	"fmt"
	"time"

	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

// newUpdater is a package-level function variable for creating an Updater.
// Tests can override this to inject a mock server.
var newUpdater func(string) *updater.Updater = updater.New

// checkUpdateHint runs a background version check with a timeout.
// Returns nil if the version is dev, the check fails, times out, or is up to date.
func checkUpdateHint(version string, timeout time.Duration) <-chan *updater.CheckResult {
	ch := make(chan *updater.CheckResult, 1)

	if updater.IsDevVersion(version) {
		close(ch)
		return ch
	}

	go func() {
		u := newUpdater(version)
		result, err := u.Check()
		if err != nil || result.UpToDate {
			close(ch)
			return
		}
		ch <- result
	}()

	// Return a channel that resolves with the result or nil after timeout
	out := make(chan *updater.CheckResult, 1)
	go func() {
		select {
		case r, ok := <-ch:
			if ok {
				out <- r
			} else {
				close(out)
			}
		case <-time.After(timeout):
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
	noColor, _ := cmd.Flags().GetBool("no-color")
	p := termcolor.NewPainter(noColor)

	msg := fmt.Sprintf(
		"Update available: %s → %s — run \"rimba update\" to upgrade",
		result.CurrentVersion, result.LatestVersion,
	)
	fmt.Fprintln(cmd.OutOrStdout(), p.Paint(msg, termcolor.Yellow))
}
