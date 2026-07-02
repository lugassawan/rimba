package git

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ctxMockRunner is an extended mock that captures the context for assertion.
type ctxMockRunner struct {
	run         func(ctx context.Context, args ...string) (string, error)
	runInDir    func(ctx context.Context, dir string, args ...string) (string, error)
	capturedCtx context.Context
}

func (m *ctxMockRunner) Run(ctx context.Context, args ...string) (string, error) {
	m.capturedCtx = ctx
	if m.run != nil {
		return m.run(ctx, args...)
	}
	return "", nil
}

func (m *ctxMockRunner) RunInDir(ctx context.Context, dir string, args ...string) (string, error) {
	m.capturedCtx = ctx
	if m.runInDir != nil {
		return m.runInDir(ctx, dir, args...)
	}
	return "", nil
}

type ctxKey struct{}

func TestFetchContextCancelledReturnsFast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel — Fetch should return immediately without launching git

	r := &ExecRunner{}
	start := time.Now()
	err := Fetch(ctx, r, "origin", FetchArgs{})
	if time.Since(start) > time.Second {
		t.Errorf("Fetch with cancelled ctx took too long: %v", time.Since(start))
	}
	if err == nil {
		t.Fatal("expected error from cancelled Fetch, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestFetchPassesContext(t *testing.T) {
	sentinel := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	r := &ctxMockRunner{}

	if err := Fetch(sentinel, r, "origin", FetchArgs{}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if r.capturedCtx == nil {
		t.Fatal("context was not captured")
	}
	if r.capturedCtx.Value(ctxKey{}) != "sentinel" {
		t.Error("sentinel context value not propagated")
	}
}

func TestRebaseContextCancelledReturnsFast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	blocker := make(chan struct{})
	r := &ctxMockRunner{
		runInDir: func(ctx context.Context, dir string, args ...string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-blocker:
				return "", nil
			}
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- Rebase(ctx, r, fakeDir, branchMain)
	}()

	// Cancel after a brief moment
	time.Sleep(10 * time.Millisecond)
	cancel()
	close(blocker)

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Rebase did not return within 1s after cancellation")
	}
}
