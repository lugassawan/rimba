package fsutil

import (
	"context"
	"testing"
)

// dirSizeWithRecoveringPanic runs dirSizeWith against a panicking walker and
// returns the panic value.
func dirSizeWithRecoveringPanic() (recovered any) {
	defer func() { recovered = recover() }()
	_, _ = dirSizeWith(context.Background(), "/ignored", func(string) (int64, error) {
		panic("walk boom")
	})
	return nil
}

func TestDirSizeRepanicsWalkerPanicOnCaller(t *testing.T) {
	if got := dirSizeWithRecoveringPanic(); got != "walk boom" {
		t.Errorf("recovered %v, want %q", got, "walk boom")
	}
}
