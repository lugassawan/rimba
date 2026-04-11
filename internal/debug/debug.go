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

// TimedRunner decorates a git.Runner, logging each command with elapsed time to stderr.
type TimedRunner struct {
	Inner git.Runner
}

func (r *TimedRunner) Run(args ...string) (string, error) {
	label := "git " + strings.Join(args, " ")
	start := time.Now()
	out, err := r.Inner.Run(args...)
	fmt.Fprintf(os.Stderr, "[debug] %s: %s\n", label, time.Since(start).Round(time.Millisecond))
	return out, err
}

func (r *TimedRunner) RunInDir(dir string, args ...string) (string, error) {
	label := fmt.Sprintf("git %s [%s]", strings.Join(args, " "), filepath.Base(dir))
	start := time.Now()
	out, err := r.Inner.RunInDir(dir, args...)
	fmt.Fprintf(os.Stderr, "[debug] %s: %s\n", label, time.Since(start).Round(time.Millisecond))
	return out, err
}

func enabled() bool {
	_, ok := os.LookupEnv("RIMBA_DEBUG")
	return ok
}
