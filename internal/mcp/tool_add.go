package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerAddTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("add",
		mcp.WithDescription("Create a new worktree for a task"),
		mcp.WithString("task",
			mcp.Description("Task identifier (e.g. 'my-feature', 'JIRA-123')"),
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
			return mcp.NewToolResultError("task is required"), nil
		}

		prefixType := req.GetString("type", "feature")

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		if !resolver.ValidPrefixType(prefixType) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid type %q; valid types: feature, bugfix, hotfix, docs, test, chore", prefixType)), nil
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

		result, err := operations.AddWorktree(hctx.Runner, operations.AddParams{
			Task:          task,
			Prefix:        prefix,
			Source:        source,
			RepoRoot:      hctx.RepoRoot,
			WorktreeDir:   filepath.Join(hctx.RepoRoot, cfg.WorktreeDir),
			CopyFiles:     cfg.CopyFiles,
			SkipDeps:      req.GetBool("skip_deps", false),
			AutoDetect:    cfg.IsAutoDetectDeps(),
			ConfigModules: configModules,
			SkipHooks:     req.GetBool("skip_hooks", false),
			PostCreate:    cfg.PostCreate,
		}, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return marshalResult(addResult{
			Task:   result.Task,
			Branch: result.Branch,
			Path:   result.Path,
			Source: result.Source,
		})
	}
}
