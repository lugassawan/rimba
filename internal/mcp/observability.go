package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// withRecorder decorates a tool handler so each call builds its own Recorder
// (bound to this one call, not the long-lived server process), attaches it
// to ctx, and finalizes + closes it before returning — the MCP-per-call
// counterpart to cmd/root.go's PersistentPreRunE-build / Execute-finalize pair.
func withRecorder(hctx *HandlerContext, toolName string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if hctx.Config == nil || !hctx.Config.IsObservabilityEnabled() {
			return handler(ctx, req)
		}
		retentionDays := hctx.Config.ObservabilityRetentionDays()
		sink, err := observability.NewFileSink(hctx.RepoRoot, retentionDays)
		if err != nil {
			return handler(ctx, req) // never block a tool call on observability failing to open
		}
		rec := observability.NewRecorder(sink, toolName, "", "", hctx.Version)
		defer rec.Close() // registered first: runs after the Finalize defer below (LIFO)

		ctx = observability.WithRecorder(ctx, rec)
		result, callErr := handler(ctx, req)

		outcome := observability.OutcomeSuccess
		if callErr != nil || (result != nil && result.IsError) {
			outcome = observability.OutcomeError
		}
		defer rec.Finalize(outcome, 0, callErr)
		return result, callErr
	}
}
