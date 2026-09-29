package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// withRecorder decorates a tool handler with a per-call Recorder, finalized and
// closed before returning — including on a handler panic, which is re-raised.
func withRecorder(hctx *HandlerContext, toolName string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, callErr error) {
		if hctx.Config == nil || !hctx.Config.IsObservabilityEnabled() {
			return handler(ctx, req)
		}
		retentionDays := hctx.Config.ObservabilityRetentionDays()
		sink, err := observability.NewFileSink(hctx.RepoRoot, retentionDays)
		if err != nil {
			return handler(ctx, req) // never block a tool call on observability failing to open
		}
		rec := observability.NewRecorder(sink, toolName, "", "", hctx.Version)
		done := false // stays false on panic or runtime.Goexit
		// recover() must be called directly inside the deferred literal to take effect.
		defer func() { finishCall(rec, recover(), done, result, callErr) }()

		result, callErr = handler(observability.WithRecorder(ctx, rec), req)
		done = true
		return result, callErr
	}
}

// finishCall finalizes and closes rec for a completed, panicked or Goexit-ed
// call, then re-raises a recovered panic so the decorator stays observation-only.
func finishCall(rec *observability.Recorder, p any, done bool, result *mcp.CallToolResult, callErr error) {
	outcome := observability.OutcomeSuccess
	recErr := callErr
	switch {
	case p != nil:
		outcome, recErr = observability.OutcomeError, observability.PanicError(p)
	case !done:
		outcome, recErr = observability.OutcomeError, observability.ErrIncomplete
	case callErr != nil || (result != nil && result.IsError):
		outcome = observability.OutcomeError
	}
	rec.Finalize(outcome, 0, recErr)
	_ = rec.Close()
	if p != nil {
		panic(p)
	}
}
