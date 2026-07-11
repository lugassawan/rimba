package mcp

import (
	"context"
	"errors"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerArchiveTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("archive",
		mcp.WithDescription("Archive a worktree: remove its directory but keep the branch for later restoration with restore"),
		mcp.WithString("task",
			mcp.Description("Task identifier to archive (e.g. 'my-task' or 'auth-api/my-task' for monorepo)"),
			mcp.Required(),
		),
		mcp.WithBoolean("force",
			mcp.Description("Force archival even if the worktree has uncommitted changes"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Preview what would be archived without making changes"),
		),
	)
	s.AddTool(tool, handleArchive(hctx))
}

func handleArchive(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawTask := req.GetString("task", "")
		if rawTask == "" {
			return errorResult(errhint.WithFix(errors.New("task is required"),
				`provide the task argument, e.g. archive { task: "my-task" }`)), nil
		}

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return errorResult(cfgErr), nil
		}

		// operations.FindWorktree resolves prefixes via
		// config.PrefixSetFromContext(ctx), which falls back to built-in defaults
		// when config is absent from ctx. Inject cfg so repos with custom
		// [[resolver.prefix]] entries resolve correctly (mirrors tool_rename.go
		// and tool_restore.go).
		ctx = config.WithConfig(ctx, cfg)

		ps := hctx.PrefixSet()
		service, task := operations.ResolveTaskInput(rawTask, hctx.RepoRoot, ps)

		wt, findErr := operations.FindWorktree(ctx, hctx.Runner, service, task)
		if findErr != nil {
			return errorResult(findErr), nil
		}

		dryRun := req.GetBool("dry_run", false)
		result, err := operations.ArchiveWorktree(ctx, hctx.Runner, operations.ArchiveParams{
			Path:   wt.Path,
			Branch: wt.Branch,
			Force:  req.GetBool("force", false),
			DryRun: dryRun,
		})
		if err != nil {
			return errorResult(err), nil
		}

		data := archiveResult{
			Path:   result.Path,
			Branch: result.Branch,
			DryRun: dryRun,
		}
		if dryRun && result.Plan != nil {
			data.Steps = result.Plan.Steps
		}

		return marshalResult(data)
	}
}
