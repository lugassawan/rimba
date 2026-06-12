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
		mcp.WithDescription("Create a new worktree for a task (supports 'service/task' for monorepo)"),
		mcp.WithString("task",
			mcp.Description("Task identifier (e.g. 'my-feature', 'JIRA-123', or 'auth-api/my-feature' for monorepo)"),
			mcp.Required(),
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

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		result, err := operations.AddWorktree(ctx, hctx.Runner, operations.AddParams{
			Task:    task,
			Service: service,
			Prefix:  prefix,
			Source:  source,
			PostCreateOptions: operations.PostCreateOptions{
				RepoRoot:      hctx.RepoRoot,
				WorktreeDir:   filepath.Join(hctx.RepoRoot, cfg.WorktreeDir),
				CopyFiles:     cfg.CopyFiles,
				SkipDeps:      req.GetBool("skip_deps", false),
				AutoDetect:    cfg.IsAutoDetectDeps(),
				ConfigModules: configModules,
				SkipHooks:     req.GetBool("skip_hooks", false),
				PostCreate:    cfg.PostCreate,
				Concurrency:   cfg.DepsConcurrency(),
			},
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
}
