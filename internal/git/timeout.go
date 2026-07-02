package git

import (
	"context"
	"time"
)

// itemQueryTimeout bounds a single git query; prevents a stalled worktree from blocking the fan-out.
const itemQueryTimeout = 10 * time.Second

// WithItemTimeout returns a child context bounded by itemQueryTimeout. Caller must defer cancel().
func WithItemTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, itemQueryTimeout)
}
