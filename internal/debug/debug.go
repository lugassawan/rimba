package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StartTimer logs the start of a labelled operation and returns a function
// that logs elapsed time when called. No-op when RIMBA_DEBUG is unset.
func StartTimer(label string) func() {
	if !enabled() {
		return func() {}
	}
	logf("%s: start", label)
	start := time.Now()
	return func() {
		logf("%s: %s", label, time.Since(start).Round(time.Millisecond))
	}
}

// LogGitTiming writes a debug timing line for a labelled git subprocess
// call, if RIMBA_DEBUG is set; no-op otherwise. Exported so the
// observability recorder's no-Recorder fallback can share this format.
func LogGitTiming(dir string, args []string, d time.Duration) {
	if !enabled() {
		return
	}
	label := "git " + strings.Join(args, " ")
	if dir != "" {
		label = fmt.Sprintf("git %s [%s]", strings.Join(args, " "), filepath.Base(dir))
	}
	logf("%s: %s", label, d.Round(time.Millisecond))
}

// logf writes a formatted debug line to stderr.
// Leading \n ensures the line is not appended to an active spinner.
func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n[debug] "+format+"\n", args...)
}

func enabled() bool {
	_, ok := os.LookupEnv("RIMBA_DEBUG")
	return ok
}
