package mcp

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerRestoreTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("restore",
		mcp.WithDescription("Restore an archived worktree from its preserved branch"),
		mcp.WithString("task",
			mcp.Description("Task identifier to restore (e.g. 'my-task' or 'auth-api/my-task' for monorepo)"),
			mcp.Required(),
		),
		mcp.WithBoolean("skip_deps",
			mcp.Description("Skip dependency detection and installation"),
		),
		mcp.WithBoolean("skip_hooks",
			mcp.Description("Skip post-create hooks"),
		),
	)
	s.AddTool(tool, handleRestore(hctx))
}

func handleRestore(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawTask := req.GetString("task", "")
		if rawTask == "" {
			return errorResult(errhint.WithFix(errors.New("task is required"),
				`provide the task argument, e.g. restore { task: "my-task" }`)), nil
		}

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return errorResult(cfgErr), nil
		}

		ps := hctx.PrefixSet()
		service, task := operations.ResolveTaskInput(rawTask, hctx.RepoRoot, ps)

		// FindArchivedBranch resolves prefixes via config.PrefixSetFromContext(ctx),
		// which falls back to built-in defaults when config is absent from ctx.
		// The CLI injects config in PersistentPreRunE; MCP handlers must do so
		// explicitly or repos with custom [[resolver.prefix]] entries get the
		// wrong prefix resolution.
		ctx = config.WithConfig(ctx, cfg)

		branch, findErr := operations.FindArchivedBranch(ctx, hctx.Runner, service, task)
		if findErr != nil {
			return errorResult(findErr), nil
		}

		if err := trust.GateNonInteractive(hctx.RepoRoot, cfg); err != nil {
			return errorResult(err), nil
		}

		wtDir := filepath.Join(hctx.RepoRoot, cfg.WorktreeDir)
		wtPath := resolver.WorktreePath(wtDir, branch)

		if err := git.AddWorktreeFromBranch(ctx, hctx.Runner, wtPath, branch); err != nil {
			return errorResult(err), nil
		}

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		pcResult, err := operations.PostCreateSetup(ctx, hctx.Runner, operations.PostCreateParams{
			RepoRoot:      hctx.RepoRoot,
			WtPath:        wtPath,
			Task:          task,
			Service:       service,
			CopyFiles:     cfg.CopyFiles,
			SkipDeps:      req.GetBool("skip_deps", false),
			AutoDetect:    cfg.IsAutoDetectDeps(),
			ConfigModules: configModules,
			SkipHooks:     req.GetBool("skip_hooks", false),
			PostCreate:    cfg.PostCreate,
			Concurrency:   cfg.DepsConcurrency(),
		}, nil)
		if err != nil {
			return errorResult(err), nil
		}

		return marshalResult(restoreResult{
			Task:            task,
			Branch:          branch,
			Path:            wtPath,
			Copied:          pcResult.Copied,
			Skipped:         pcResult.Skipped,
			SkippedSymlinks: pcResult.SkippedSymlinks,
		})
	}
}
