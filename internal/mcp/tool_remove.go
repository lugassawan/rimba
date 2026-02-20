package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerRemoveTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("remove",
		mcp.WithDescription("Remove a worktree and optionally delete its branch"),
		mcp.WithString("task",
			mcp.Description("Task identifier of the worktree to remove"),
			mcp.Required(),
		),
		mcp.WithBoolean("keep_branch",
			mcp.Description("Keep the local branch after removing the worktree"),
		),
		mcp.WithBoolean("force",
			mcp.Description("Force removal even if the worktree has uncommitted changes"),
		),
	)
	s.AddTool(tool, handleRemove(hctx))
}

func handleRemove(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		task := req.GetString("task", "")
		if task == "" {
			return mcp.NewToolResultError("task is required"), nil
		}

		keepBranch := req.GetBool("keep_branch", false)
		force := req.GetBool("force", false)

		r := hctx.Runner

		wt, findErr := operations.FindWorktree(r, task)
		if findErr != nil {
			return mcp.NewToolResultError(findErr.Error()), nil
		}

		if err := git.RemoveWorktree(r, wt.Path, force); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result := removeResult{
			Task:            task,
			Branch:          wt.Branch,
			WorktreeRemoved: true,
			BranchDeleted:   false,
		}

		if !keepBranch {
			if err := git.DeleteBranch(r, wt.Branch, true); err != nil {
				// Worktree removed but branch deletion failed — partial success
				return marshalResult(result)
			}
			result.BranchDeleted = true
		}

		return marshalResult(result)
	}
}
