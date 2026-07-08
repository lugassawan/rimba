package mcp

import (
	"context"
	"errors"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMergeTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("merge",
		mcp.WithDescription("Merge a worktree branch into main or another worktree (supports 'service/task' for monorepo)"),
		mcp.WithString("source",
			mcp.Description("Source task to merge (e.g. 'my-task' or 'auth-api/my-task' for monorepo)"),
			mcp.Required(),
		),
		mcp.WithString("into",
			mcp.Description("Target task to merge into (default: main branch; supports 'service/task' for monorepo)"),
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
			return errorResult(errhint.WithFix(errors.New("source is required"),
				`provide the source argument, e.g. merge { source: "my-feature" }`)), nil
		}

		ps := hctx.PrefixSet()
		sourceService, sourceTask := operations.ResolveTaskInput(sourceTask, hctx.RepoRoot, ps)

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return errorResult(cfgErr), nil
		}

		source, findErr := operations.FindWorktree(ctx, hctx.Runner, sourceService, sourceTask)
		if findErr != nil {
			return errorResult(findErr), nil
		}
		if err := operations.GuardKnownPrefix(ps, source.Branch, cfg.DefaultSource, false); err != nil {
			return errorResult(err), nil
		}

		intoTask := req.GetString("into", "")
		var intoService string
		if intoTask != "" {
			intoService, intoTask = operations.ResolveTaskInput(intoTask, hctx.RepoRoot, ps)
		}

		result, err := operations.MergeWorktree(ctx, hctx.Runner, operations.MergeParams{
			SourceTask:    sourceTask,
			SourceService: sourceService,
			IntoTask:      intoTask,
			IntoService:   intoService,
			RepoRoot:      hctx.RepoRoot,
			MainBranch:    cfg.DefaultSource,
			NoFF:          req.GetBool("no_ff", false),
			Keep:          req.GetBool("keep", false),
			Delete:        req.GetBool("delete", false),
		}, nil)
		if err != nil {
			return errorResult(err), nil
		}

		return marshalResult(mergeResult{
			Source:        result.SourceBranch,
			Into:          result.TargetLabel,
			SourceRemoved: result.SourceRemoved,
			RemoteDeleted: result.RemoteDeleted,
		})
	}
}
