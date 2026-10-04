package mcp

import (
	"context"
	"errors"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDuplicateTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("duplicate",
		mcp.WithDescription("Create a new worktree from an existing worktree's branch"),
		mcp.WithString("task",
			mcp.Description("Task identifier to duplicate (e.g. 'my-task' or 'auth-api/my-task' for monorepo)"),
			mcp.Required(),
		),
		mcp.WithString("as", mcp.Description("Custom name for the duplicate worktree")),
		mcp.WithBoolean("skip_deps", mcp.Description("Skip dependency detection and installation")),
		mcp.WithBoolean("skip_hooks", mcp.Description("Skip post-create hooks")),
		mcp.WithBoolean("dry_run", mcp.Description("Preview what would be duplicated without making changes")),
	)
	s.AddTool(tool, withRecorder(hctx, "duplicate", handleDuplicate(hctx)))
}

func handleDuplicate(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawTask := req.GetString("task", "")
		if rawTask == "" {
			return errorResult(errhint.WithFix(errors.New("task is required"),
				`provide the task argument, e.g. duplicate { task: "my-task" }`)), nil
		}
		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return errorResult(cfgErr), nil
		}
		ctx = config.WithConfig(ctx, cfg)
		ps := hctx.PrefixSet()
		service, task := operations.ResolveTaskInput(rawTask, hctx.RepoRoot, ps)
		source, err := operations.FindWorktree(ctx, hctx.Runner, service, task)
		if err != nil {
			return errorResult(err), nil
		}

		dryRun := req.GetBool("dry_run", false)
		if !dryRun {
			if err := trust.GateNonInteractive(hctx.RepoRoot, cfg); err != nil {
				return errorResult(err), nil
			}
		}
		result, err := operations.DuplicateWorktree(ctx, hctx.Runner, operations.DuplicateParams{
			Source:            source,
			SourceTask:        task,
			As:                req.GetString("as", ""),
			PrefixSet:         ps,
			DefaultSource:     cfg.DefaultSource,
			DryRun:            dryRun,
			PostCreateOptions: buildPostCreateOptions(hctx, cfg, req),
		}, nil)
		if err != nil {
			return errorResult(err), nil
		}
		return marshalResult(duplicateResult{
			SourceTask:      result.SourceTask,
			SourceBranch:    result.SourceBranch,
			Task:            result.Task,
			Branch:          result.Branch,
			Path:            result.Path,
			DryRun:          result.DryRun,
			Copied:          result.Copied,
			Skipped:         result.Skipped,
			SkippedSymlinks: result.SkippedSymlinks,
		})
	}
}
