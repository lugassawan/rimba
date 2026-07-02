package git

import (
	"context"
	"testing"
	"time"
)

func TestWithItemTimeoutDeadlineIsSet(t *testing.T) {
	ctx, cancel := WithItemTimeout(context.Background())
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}
	remaining := time.Until(dl)
	if remaining <= 0 || remaining > itemQueryTimeout {
		t.Errorf("deadline out of range: %v (want 0 < d <= %v)", remaining, itemQueryTimeout)
	}
}

func TestWithItemTimeoutCancelledParentPropagates(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel() // pre-cancel

	ctx, cancel := WithItemTimeout(parent)
	defer cancel()

	select {
	case <-ctx.Done():
		// expected: parent cancellation flows through
	case <-time.After(100 * time.Millisecond):
		t.Error("expected cancelled parent to cancel child context immediately")
	}
}
