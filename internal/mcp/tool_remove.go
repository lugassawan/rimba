package mcp

import (
	"context"
	"errors"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerRemoveTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("remove",
		mcp.WithDescription("Remove a worktree and optionally delete its branch (supports 'service/task' for monorepo)"),
		mcp.WithString("task",
			mcp.Description("Task identifier of the worktree to remove (e.g. 'my-task' or 'auth-api/my-task' for monorepo)"),
			mcp.Required(),
		),
		mcp.WithBoolean("keep_branch",
			mcp.Description("Keep the local branch after removing the worktree"),
		),
		mcp.WithBoolean("force",
			mcp.Description("Force removal even if the worktree has uncommitted changes"),
		),
	)
	s.AddTool(tool, withRecorder(hctx, "remove", handleRemove(hctx)))
}

func handleRemove(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		task := req.GetString("task", "")
		if task == "" {
			return errorResult(errhint.WithFix(errors.New("task is required"),
				`provide the task argument, e.g. remove { task: "my-task" }`)), nil
		}

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return errorResult(cfgErr), nil
		}

		ps := hctx.PrefixSet()
		service, task := operations.ResolveTaskInput(task, hctx.RepoRoot, ps)

		keepBranch := req.GetBool("keep_branch", false)
		force := req.GetBool("force", false)

		r := hctx.Runner

		wt, findErr := operations.FindWorktree(ctx, r, service, task)
		if findErr != nil {
			return errorResult(findErr), nil
		}

		if err := operations.GuardKnownPrefix(ps, wt.Branch, cfg.DefaultSource, force); err != nil {
			return errorResult(err), nil
		}

		result, err := operations.RemoveWorktree(ctx, r, wt, task, keepBranch, force, nil)
		if err != nil {
			return errorResult(err), nil
		}

		return marshalResult(removeResult{
			Task:            result.Task,
			Branch:          result.Branch,
			WorktreeRemoved: result.WorktreeRemoved,
			BranchDeleted:   result.BranchDeleted,
		})
	}
}
