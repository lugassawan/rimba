package git

import (
	"context"
	"time"
)

// itemQueryTimeout caps a single git status/log/count query so one stalled
// worktree (e.g. NFS mount) cannot hang a fan-out indefinitely.
const itemQueryTimeout = 10 * time.Second

// WithItemTimeout derives a child context bounded by itemQueryTimeout.
// Callers must defer the returned cancel to release the timer.
func WithItemTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, itemQueryTimeout)
}
