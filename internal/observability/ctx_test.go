package observability

import (
	"context"
	"testing"
)

func TestWithRecorderRoundTrip(t *testing.T) {
	rec := &Recorder{command: "test-cmd"}
	ctx := WithRecorder(context.Background(), rec)

	got := FromContext(ctx)
	if got != rec {
		t.Errorf("FromContext() = %v, want %v", got, rec)
	}
}

func TestFromContextAbsent(t *testing.T) {
	got := FromContext(context.Background())
	if got != nil {
		t.Errorf("FromContext(bare context) = %v, want nil", got)
	}
}
