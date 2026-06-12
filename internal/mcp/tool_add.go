package mcp

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

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
			mcp.Description("Task identifier (e.g. 'my-feature', 'JIRA-123', or 'auth-api/my-feature' for monorepo); required for normal add, optional as PR name override when pr is set"),
		),
		mcp.WithString("pr",
			mcp.Description("GitHub PR number to open a review worktree from (branch review/<num>-<slug>); requires gh authenticated"),
		),
		mcp.WithString("branch",
			mcp.Description("Existing, currently checked-out local branch to promote into its own worktree (stash-transfers dirty state)"),
		),
		mcp.WithString("type",
			mcp.Description("Prefix type (default: feature)"),
			mcp.Enum("feature", "bugfix", "hotfix", "docs", "test", "chore"),
		),
		mcp.WithString("source",
			mcp.Description("Source branch to create worktree from (default from config)"),
		),
		mcp.WithBoolean("skip_deps",
			mcp.Description("Skip dependency installation"),
		),
		mcp.WithBoolean("skip_hooks",
			mcp.Description("Skip post-create hooks"),
		),
	)
	s.AddTool(tool, handleAdd(hctx))
}

func handleAdd(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prStr := req.GetString("pr", "")
		branch := req.GetString("branch", "")

		if prStr != "" && branch != "" {
			return errorResult(errhint.WithFix(
				errors.New("pr and branch are mutually exclusive"),
				"provide either pr or branch, not both",
			)), nil
		}

		switch {
		case prStr != "":
			return handleAddPR(ctx, hctx, req, prStr)
		case branch != "":
			return handleAddBranch(ctx, hctx, req, branch)
		default:
			return handleAddTask(ctx, hctx, req)
		}
	}
}

func handleAddTask(ctx context.Context, hctx *HandlerContext, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	task := req.GetString("task", "")
	if task == "" {
		return errorResult(errhint.WithFix(errors.New("task is required"),
			`provide the task argument, e.g. add { task: "my-feature" }`)), nil
	}

	service, task := operations.ResolveTaskInput(task, hctx.RepoRoot)

	prefixType := req.GetString("type", "feature")

	cfg, cfgErr := hctx.requireConfig()
	if cfgErr != nil {
		return errorResult(cfgErr), nil
	}

	if err := trust.GateNonInteractive(hctx.RepoRoot, cfg); err != nil {
		return errorResult(err), nil
	}

	if !resolver.ValidPrefixType(prefixType) {
		return errorResult(errhint.WithFix(
			fmt.Errorf("invalid type %q", prefixType),
			"use one of: feature, bugfix, hotfix, docs, test, chore (or omit to default to feature)",
		)), nil
	}

	prefix, _ := resolver.PrefixString(resolver.PrefixType(prefixType))

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

func handleAddPR(ctx context.Context, hctx *HandlerContext, req mcp.CallToolRequest, prStr string) (*mcp.CallToolResult, error) {
	prNum, _ := strconv.Atoi(prStr)
	if prNum <= 0 {
		return errorResult(errhint.WithFix(
			fmt.Errorf("invalid pr number %q", prStr),
			"provide a positive integer, e.g. add { pr: \"42\" }",
		)), nil
	}

	cfg, cfgErr := hctx.requireConfig()
	if cfgErr != nil {
		return errorResult(cfgErr), nil
	}

	if err := trust.GateNonInteractive(hctx.RepoRoot, cfg); err != nil {
		return errorResult(err), nil
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

func handleAddBranch(ctx context.Context, hctx *HandlerContext, _ mcp.CallToolRequest, branch string) (*mcp.CallToolResult, error) {
	cfg, cfgErr := hctx.requireConfig()
	if cfgErr != nil {
		return errorResult(cfgErr), nil
	}

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
