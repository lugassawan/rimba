package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/operations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMergeTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("merge",
		mcp.WithDescription("Merge a worktree branch into main or another worktree"),
		mcp.WithString("source",
			mcp.Description("Source task to merge"),
			mcp.Required(),
		),
		mcp.WithString("into",
			mcp.Description("Target task to merge into (default: main branch)"),
		),
		mcp.WithBoolean("no_ff",
			mcp.Description("Force a merge commit (no fast-forward)"),
		),
		mcp.WithBoolean("keep",
			mcp.Description("Keep source worktree after merging into main"),
		),
		mcp.WithBoolean("delete",
			mcp.Description("Delete source worktree after merging into another worktree"),
		),
	)
	s.AddTool(tool, handleMerge(hctx))
}

func handleMerge(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sourceTask := req.GetString("source", "")
		if sourceTask == "" {
			return mcp.NewToolResultError("source is required"), nil
		}

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		result, err := operations.MergeWorktree(hctx.Runner, operations.MergeParams{
			SourceTask: sourceTask,
			IntoTask:   req.GetString("into", ""),
			RepoRoot:   hctx.RepoRoot,
			MainBranch: cfg.DefaultSource,
			NoFF:       req.GetBool("no_ff", false),
			Keep:       req.GetBool("keep", false),
			Delete:     req.GetBool("delete", false),
		}, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(mergeResult{
			Source:        result.SourceBranch,
			Into:          result.TargetLabel,
			SourceRemoved: result.SourceRemoved,
		})
	}
}
