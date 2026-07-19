package mcp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAddTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("add",
		mcp.WithDescription("Create a new worktree for a task, a GitHub PR review, or promote an existing local branch"),
		mcp.WithString("task",
			mcp.Description("Task identifier (e.g. 'my-feature', 'JIRA-123', or 'auth-api/my-feature' for monorepo); required for normal add, optional as PR name override when pr is set; ignored in branch mode"),
		),
		mcp.WithInteger("pr",
			mcp.Description("GitHub PR number to open a review worktree from (branch review/<num>-<slug>); requires gh authenticated"),
		),
		mcp.WithString("branch",
			mcp.Description("Existing, currently checked-out local branch to promote into its own worktree (stash-transfers dirty state)"),
		),
		mcp.WithString("type",
			mcp.Description("Prefix type (default: feature)"),
		),
		mcp.WithString("source",
			mcp.Description("Source branch to create worktree from (default from config); applies to task mode only"),
		),
		mcp.WithBoolean("skip_deps",
			mcp.Description("Skip dependency installation (applies to task and pr modes)"),
		),
		mcp.WithBoolean("skip_hooks",
			mcp.Description("Skip post-create hooks (applies to task and pr modes)"),
		),
	)
	s.AddTool(tool, withRecorder(hctx, "add", handleAdd(hctx)))
}

func handleAdd(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prNum := req.GetInt("pr", 0)
		branch := req.GetString("branch", "")

		if prNum != 0 && branch != "" {
			return errorResult(errhint.WithFix(
				errors.New("pr and branch are mutually exclusive"),
				"provide either pr or branch, not both",
			)), nil
		}

		switch {
		// prNum != 0 intentionally routes negative values to handleAddPR,
		// which validates prNum > 0 and returns a clear "invalid pr number" error.
		case prNum != 0:
			return handleAddPR(ctx, hctx, req, prNum)
		case branch != "":
			return handleAddBranch(ctx, hctx, branch)
		default:
			return handleAddTask(ctx, hctx, req)
		}
	}
}

func handleAddTask(ctx context.Context, hctx *HandlerContext, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rawTask := req.GetString("task", "")
	if rawTask == "" {
		return errorResult(errhint.WithFix(errors.New("task is required"),
			`provide the task argument, e.g. add { task: "my-feature" }`)), nil
	}

	ps := hctx.PrefixSet()
	service, task := operations.ResolveTaskInput(rawTask, hctx.RepoRoot, ps)

	prefixType := resolveMCPPrefixType(req, rawTask, ps)

	cfg, cfgErr := hctx.requireConfig()
	if cfgErr != nil {
		return errorResult(cfgErr), nil
	}

	if err := trust.GateNonInteractive(hctx.RepoRoot, cfg); err != nil {
		return errorResult(err), nil
	}

	if !ps.ValidType(prefixType) {
		return invalidTypeResult(prefixType, ps, " (or omit to default to feature)"), nil
	}

	prefix, _ := ps.TypeToPrefix(prefixType)

	source := req.GetString("source", "")
	if source == "" {
		source = cfg.DefaultSource
	}

	result, err := operations.AddWorktree(ctx, hctx.Runner, operations.AddParams{
		Task:              task,
		Service:           service,
		Prefix:            prefix,
		Source:            source,
		PostCreateOptions: buildPostCreateOptions(hctx, cfg, req),
	}, nil)
	if err != nil {
		return errorResult(err), nil
	}

	return marshalResult(addResult{
		Task:   result.Task,
		Branch: result.Branch,
		Path:   result.Path,
		Source: result.Source,
	})
}

// resolveMCPPrefixType falls back to rawTask's leading segment (mirroring
// cmd/add.go's resolveAddPrefix) when "type" is omitted; the "type" enum
// itself stays canonical-only.
func resolveMCPPrefixType(req mcp.CallToolRequest, rawTask string, ps *resolver.PrefixSet) string {
	if t := req.GetString("type", ""); t != "" {
		return t
	}

	if candidate, _ := resolver.SplitServiceInput(rawTask); candidate != "" {
		if prefix, _, ok := ps.TokenToPrefix(candidate); ok {
			return ps.TypeName(prefix)
		}
	}

	return string(resolver.DefaultPrefixType)
}

func handleAddPR(ctx context.Context, hctx *HandlerContext, req mcp.CallToolRequest, prNum int) (*mcp.CallToolResult, error) {
	if prNum <= 0 {
		return errorResult(errhint.WithFix(
			fmt.Errorf("invalid pr number %d", prNum),
			"provide a positive integer, e.g. add { \"pr\": 42 }",
		)), nil
	}

	cfg, cfgErr := hctx.requireConfig()
	if cfgErr != nil {
		return errorResult(cfgErr), nil
	}

	if err := trust.GateNonInteractive(hctx.RepoRoot, cfg); err != nil {
		return errorResult(err), nil
	}

	if hctx.GH == nil {
		return errorResult(errors.New("gh runner not configured; this is a server startup bug")), nil
	}

	result, err := operations.AddPRWorktree(ctx, hctx.Runner, hctx.GH, operations.AddPRParams{
		PRNumber:          prNum,
		TaskOverride:      req.GetString("task", ""),
		PostCreateOptions: buildPostCreateOptions(hctx, cfg, req),
	}, nil)
	if err != nil {
		return errorResult(err), nil
	}

	return marshalResult(addResult{
		Task:   result.Task,
		Branch: result.Branch,
		Path:   result.Path,
		Source: result.Source,
	})
}

func handleAddBranch(ctx context.Context, hctx *HandlerContext, branch string) (*mcp.CallToolResult, error) {
	cfg, cfgErr := hctx.requireConfig()
	if cfgErr != nil {
		return errorResult(cfgErr), nil
	}

	// No trust gate: PromoteBranch runs no post-create hooks, matching CLI branch: mode.
	wtDir := filepath.Join(hctx.RepoRoot, cfg.WorktreeDir)
	path, err := operations.PromoteBranch(ctx, wtDir, hctx.Runner, hctx.RepoRoot, branch)
	if err != nil {
		return errorResult(err), nil
	}

	return marshalResult(addResult{
		Branch: branch,
		Path:   path,
	})
}

func buildPostCreateOptions(hctx *HandlerContext, cfg *config.Config, req mcp.CallToolRequest) operations.PostCreateOptions {
	var configModules []config.ModuleConfig
	if cfg.Deps != nil {
		configModules = cfg.Deps.Modules
	}
	return operations.PostCreateOptions{
		RepoRoot:      hctx.RepoRoot,
		WorktreeDir:   filepath.Join(hctx.RepoRoot, cfg.WorktreeDir),
		CopyFiles:     cfg.CopyFiles,
		SkipDeps:      req.GetBool("skip_deps", false),
		AutoDetect:    cfg.IsAutoDetectDeps(),
		ConfigModules: configModules,
		SkipHooks:     req.GetBool("skip_hooks", false),
		PostCreate:    cfg.PostCreate,
		Concurrency:   cfg.DepsConcurrency(),
	}
}
