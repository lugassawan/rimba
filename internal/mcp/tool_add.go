package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
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
		source := req.GetString("source", "")
		skipDeps := req.GetBool("skip_deps", false)
		skipHooks := req.GetBool("skip_hooks", false)

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		r := hctx.Runner

		if !resolver.ValidPrefixType(prefixType) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid type %q; valid types: feature, bugfix, hotfix, docs, test, chore", prefixType)), nil
		}

		prefix, _ := resolver.PrefixString(resolver.PrefixType(prefixType))

		if source == "" {
			source = cfg.DefaultSource
		}

		branch := resolver.BranchName(prefix, task)
		wtDir := filepath.Join(hctx.RepoRoot, cfg.WorktreeDir)
		wtPath := resolver.WorktreePath(wtDir, branch)

		// Validate
		if git.BranchExists(r, branch) {
			return mcp.NewToolResultError(fmt.Sprintf("branch %q already exists", branch)), nil
		}
		if _, err := os.Stat(wtPath); err == nil {
			return mcp.NewToolResultError("worktree path already exists: " + wtPath), nil
		}

		// Create worktree
		if err := git.AddWorktree(r, wtPath, branch, source); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Copy files
		if _, err := fileutil.CopyEntries(hctx.RepoRoot, wtPath, cfg.CopyFiles); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("worktree created but failed to copy files: %v\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, wtPath, task)), nil
		}

		// Dependencies
		if !skipDeps {
			wtEntries, _ := git.ListWorktrees(r)
			wtPaths := worktreePathsExcluding(wtEntries, wtPath)
			modules, err := deps.ResolveModules(wtPath, cfg.IsAutoDetectDeps(), configModules(cfg), wtPaths)
			if err == nil && len(modules) > 0 {
				mgr := &deps.Manager{Runner: r}
				mgr.Install(wtPath, modules, nil)
			}
		}

		// Post-create hooks
		if !skipHooks && len(cfg.PostCreate) > 0 {
			deps.RunPostCreateHooks(wtPath, cfg.PostCreate, nil)
		}

		return marshalResult(addResult{
			Task:   task,
			Branch: branch,
			Path:   wtPath,
			Source: source,
		})
	}
}

// worktreePathsExcluding returns paths for all worktree entries except the given one.
func worktreePathsExcluding(entries []git.WorktreeEntry, exclude string) []string {
	var paths []string
	for _, e := range entries {
		if e.Path != exclude {
			paths = append(paths, e.Path)
		}
	}
	return paths
}

// configModules converts config deps modules to the slice expected by deps.ResolveModules.
func configModules(cfg *config.Config) []config.ModuleConfig {
	if cfg.Deps == nil {
		return nil
	}
	return cfg.Deps.Modules
}
