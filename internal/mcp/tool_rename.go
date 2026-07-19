package mcp

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerRenameTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("rename",
		mcp.WithDescription("Rename a worktree's task, branch, and directory; optionally retype it or publish the rename to the remote"),
		mcp.WithString("task",
			mcp.Description("Task identifier to rename (e.g. 'my-task' or 'auth-api/my-task' for monorepo)"),
			mcp.Required(),
		),
		mcp.WithString("new_task",
			mcp.Description("New task name (default: same as task, for retype-only renames)"),
		),
		mcp.WithString("type",
			mcp.Description("New prefix type (default: inherit the worktree's current type)"),
		),
		mcp.WithBoolean("force",
			mcp.Description("Force rename even if the worktree is locked or uses an unconfigured prefix"),
		),
		mcp.WithBoolean("push",
			mcp.Description("Publish the renamed branch and delete the old remote branch"),
		),
		mcp.WithBoolean("skip_deps",
			mcp.Description("Skip dependency refresh after rename"),
		),
		mcp.WithBoolean("skip_hooks",
			mcp.Description("Skip post-rename hooks"),
		),
	)
	s.AddTool(tool, withRecorder(hctx, "rename", handleRename(hctx)))
}

func handleRename(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawTask := req.GetString("task", "")
		if rawTask == "" {
			return errorResult(errhint.WithFix(errors.New("task is required"),
				`provide the task argument, e.g. rename { task: "my-task" }`)), nil
		}

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return errorResult(cfgErr), nil
		}

		// Inject cfg: FindWorktree/RenameWorktree/PostRenameSetup read prefixes from
		// ctx and otherwise fall back to built-ins, breaking custom prefixes.
		ctx = config.WithConfig(ctx, cfg)

		ps := hctx.PrefixSet()
		force := req.GetBool("force", false)

		wt, findErrResult := findRenameTarget(ctx, hctx, cfg, ps, rawTask, force)
		if findErrResult != nil {
			return findErrResult, nil
		}

		newPrefix, typeErr := resolveRenamePrefix(req, ps)
		if typeErr != nil {
			return typeErr, nil
		}

		if err := trust.GateNonInteractive(hctx.RepoRoot, cfg); err != nil {
			return errorResult(err), nil
		}

		target := renameTarget{
			WT:        wt,
			NewTask:   resolveRenameNewTask(req, hctx, ps, rawTask),
			NewPrefix: newPrefix,
			Force:     force,
		}
		result, renameErrResult := performRename(ctx, hctx, cfg, req, ps, target)
		if renameErrResult != nil {
			return renameErrResult, nil
		}

		return marshalResult(renameResult{
			OldBranch:      result.OldBranch,
			NewBranch:      result.NewBranch,
			OldPath:        result.OldPath,
			NewPath:        result.NewPath,
			Published:      result.Published,
			PublishError:   errString(result.PublishError),
			RemoteDeleted:  result.RemoteDeleted,
			RemoteError:    errString(result.RemoteError),
			NoOriginRemote: result.NoOriginRemote,
		})
	}
}

// findRenameTarget resolves rawTask to a worktree and guards its prefix is still configured.
func findRenameTarget(ctx context.Context, hctx *HandlerContext, cfg *config.Config, ps *resolver.PrefixSet, rawTask string, force bool) (resolver.WorktreeInfo, *mcp.CallToolResult) {
	service, task := operations.ResolveTaskInput(rawTask, hctx.RepoRoot, ps)

	wt, findErr := operations.FindWorktree(ctx, hctx.Runner, service, task)
	if findErr != nil {
		return resolver.WorktreeInfo{}, errorResult(findErr)
	}

	if err := operations.GuardKnownPrefix(ps, wt.Branch, cfg.DefaultSource, force); err != nil {
		return resolver.WorktreeInfo{}, errorResult(err)
	}

	return wt, nil
}

// resolveRenameNewTask defaults new_task to rawTask for retype-only renames.
func resolveRenameNewTask(req mcp.CallToolRequest, hctx *HandlerContext, ps *resolver.PrefixSet, rawTask string) string {
	rawNewTask := req.GetString("new_task", "")
	if rawNewTask == "" {
		rawNewTask = rawTask
	}
	_, newTask := operations.ResolveTaskInput(rawNewTask, hctx.RepoRoot, ps)
	return newTask
}

// renameTarget bundles the resolved rename destination and options.
type renameTarget struct {
	WT        resolver.WorktreeInfo
	NewTask   string
	NewPrefix string
	Force     bool
}

// performRename renames the worktree and runs post-rename setup.
func performRename(ctx context.Context, hctx *HandlerContext, cfg *config.Config, req mcp.CallToolRequest, ps *resolver.PrefixSet, target renameTarget) (operations.RenameResult, *mcp.CallToolResult) {
	wtDir := filepath.Join(hctx.RepoRoot, cfg.WorktreeDir)

	result, err := operations.RenameWorktree(ctx, hctx.Runner, operations.RenameParams{
		WT:        target.WT,
		NewTask:   target.NewTask,
		NewPrefix: target.NewPrefix,
		WtDir:     wtDir,
		Force:     target.Force,
		Push:      req.GetBool("push", false),
	})
	if err != nil {
		return operations.RenameResult{}, errorResult(err)
	}

	if err := runPostRenameSetup(ctx, hctx, cfg, req, ps, result); err != nil {
		return operations.RenameResult{}, errorResult(err)
	}

	return result, nil
}

// resolveRenamePrefix maps the optional "type" arg to a prefix; empty means inherit.
func resolveRenamePrefix(req mcp.CallToolRequest, ps *resolver.PrefixSet) (string, *mcp.CallToolResult) {
	typeName := req.GetString("type", "")
	if typeName == "" {
		return "", nil
	}
	if !ps.ValidType(typeName) {
		return "", invalidTypeResult(typeName, ps, " (or omit to inherit the current type)")
	}
	newPrefix, _ := ps.TypeToPrefix(typeName)
	return newPrefix, nil
}

// runPostRenameSetup refreshes dependencies and runs post-rename hooks.
func runPostRenameSetup(ctx context.Context, hctx *HandlerContext, cfg *config.Config, req mcp.CallToolRequest, ps *resolver.PrefixSet, result operations.RenameResult) error {
	svc, _, _ := resolver.ServiceFromBranch(result.NewBranch, ps.Strip())
	var configModules []config.ModuleConfig
	if cfg.Deps != nil {
		configModules = cfg.Deps.Modules
	}
	_, err := operations.PostRenameSetup(ctx, hctx.Runner, operations.PostRenameParams{
		WtPath:        result.NewPath,
		Service:       svc,
		SkipDeps:      req.GetBool("skip_deps", false),
		AutoDetect:    cfg.IsAutoDetectDeps(),
		ConfigModules: configModules,
		SkipHooks:     req.GetBool("skip_hooks", false),
		PostRename:    cfg.PostRename,
		Concurrency:   cfg.DepsConcurrency(),
	}, nil)
	return err
}

// errString returns err.Error(), or "" if err is nil, for JSON string,omitempty fields.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
