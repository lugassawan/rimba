package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/git"
)

// WrapRunner returns a TimedRunner when RIMBA_DEBUG is set, otherwise returns r unchanged.
func WrapRunner(r git.Runner) git.Runner {
	if !enabled() {
		return r
	}
	return &TimedRunner{Inner: r}
}

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

// TimedRunner decorates a git.Runner, logging each command with elapsed time to stderr.
type TimedRunner struct {
	Inner git.Runner
}

func (r *TimedRunner) Run(args ...string) (string, error) {
	label := "git " + strings.Join(args, " ")
	start := time.Now()
	out, err := r.Inner.Run(args...)
	logf("%s: %s", label, time.Since(start).Round(time.Millisecond))
	return out, err
}

func (r *TimedRunner) RunInDir(dir string, args ...string) (string, error) {
	label := fmt.Sprintf("git %s [%s]", strings.Join(args, " "), filepath.Base(dir))
	start := time.Now()
	out, err := r.Inner.RunInDir(dir, args...)
	logf("%s: %s", label, time.Since(start).Round(time.Millisecond))
	return out, err
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
