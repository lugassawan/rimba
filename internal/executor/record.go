package executor

import (
	"context"
	"time"

	"github.com/lugassawan/rimba/internal/observability"
)

// WrapRunFunc decorates fn so every invocation is recorded via rec. Returns fn
// unchanged when rec is nil.
func WrapRunFunc(fn RunFunc, rec *observability.Recorder) RunFunc {
	if rec == nil {
		return fn
	}
	return func(ctx context.Context, dir, command string) ([]byte, []byte, int, error) {
		start := time.Now()
		stdout, stderr, exitCode, err := fn(ctx, dir, command)
		failed := err != nil || exitCode != 0
		stderrStr := ""
		if failed {
			stderrStr = string(stderr)
		}
		rec.LogSubprocess(observability.CategoryExec, dir, []string{command}, exitCode, time.Since(start), stderrStr, failed)
		return stdout, stderr, exitCode, err
	}
}
