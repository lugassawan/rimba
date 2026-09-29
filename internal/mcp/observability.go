package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// withRecorder wraps a tool handler in a per-call Recorder over the shared
// hctx.Sink, finalized even on panic (re-raised); it never closes the sink.
func withRecorder(hctx *HandlerContext, toolName string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, callErr error) {
		if hctx.Sink == nil {
			return handler(ctx, req)
		}
		rec := observability.NewRecorder(hctx.Sink, toolName, "", "", hctx.Version)
		done := false // stays false on panic or runtime.Goexit
		// recover() must be called directly inside the deferred literal to take effect.
		defer func() { finishCall(rec, recover(), done, result, callErr) }()

		result, callErr = handler(observability.WithRecorder(ctx, rec), req)
		done = true
		return result, callErr
	}
}

// finishCall finalizes rec for a completed, panicked or Goexit-ed
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
	if p != nil {
		panic(p)
	}
}
