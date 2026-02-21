package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSyncTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("sync",
		mcp.WithDescription("Sync worktree(s) with the main branch via rebase or merge, then push"),
		mcp.WithString("task",
			mcp.Description("Single task to sync"),
		),
		mcp.WithBoolean("all",
			mcp.Description("Sync all eligible worktrees"),
		),
		mcp.WithBoolean("merge",
			mcp.Description("Use merge instead of rebase"),
		),
		mcp.WithBoolean("include_inherited",
			mcp.Description("Include inherited/duplicate worktrees when using all"),
		),
		mcp.WithBoolean("no_push",
			mcp.Description("Skip pushing after sync"),
		),
	)
	s.AddTool(tool, handleSync(hctx))
}

// syncOpts bundles shared sync configuration derived from a single request.
type syncOpts struct {
	mainBranch   string
	useMerge     bool
	push         bool
	fetchWarning string
}

func handleSync(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		task := req.GetString("task", "")
		all := req.GetBool("all", false)
		useMerge := req.GetBool("merge", false)
		includeInherited := req.GetBool("include_inherited", false)
		noPush := req.GetBool("no_push", false)

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		if !all && task == "" {
			return mcp.NewToolResultError("provide a task name or set all=true to sync all worktrees"), nil
		}

		r := hctx.Runner

		// Fetch (non-fatal)
		var fetchWarning string
		if err := git.Fetch(r, "origin"); err != nil {
			fetchWarning = "fetch failed (no remote?): continuing with local state"
		}

		worktrees, err := operations.ListWorktreeInfos(r)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		prefixes := resolver.AllPrefixes()
		opts := syncOpts{
			mainBranch:   cfg.DefaultSource,
			useMerge:     useMerge,
			push:         !noPush,
			fetchWarning: fetchWarning,
		}

		if !all {
			return syncSingle(r, task, worktrees, prefixes, opts)
		}
		return syncMultiple(r, worktrees, prefixes, opts, includeInherited)
	}
}

func syncSingle(r git.Runner, task string, worktrees []resolver.WorktreeInfo, prefixes []string, opts syncOpts) (*mcp.CallToolResult, error) {
	wt, found := resolver.FindBranchForTask(task, worktrees, prefixes)
	if !found {
		return mcp.NewToolResultError("worktree not found for task \"" + task + "\""), nil
	}

	sr := operations.SyncWorktree(r, opts.mainBranch, wt, opts.useMerge, opts.push)

	results := []syncWorktreeResult{convertSyncResult(sr)}

	return marshalResult(syncResult{FetchWarning: opts.fetchWarning, Results: results})
}

func syncMultiple(r git.Runner, worktrees []resolver.WorktreeInfo, prefixes []string, opts syncOpts, includeInherited bool) (*mcp.CallToolResult, error) {
	allTasks := operations.CollectTasks(worktrees, prefixes)
	eligible := operations.FilterEligible(worktrees, prefixes, opts.mainBranch, allTasks, includeInherited)

	results := parallel.Collect(len(eligible), 4, func(i int) syncWorktreeResult {
		return convertSyncResult(operations.SyncWorktree(r, opts.mainBranch, eligible[i], opts.useMerge, opts.push))
	})

	return marshalResult(syncResult{FetchWarning: opts.fetchWarning, Results: results})
}

func convertSyncResult(sr operations.SyncWorktreeResult) syncWorktreeResult {
	return syncWorktreeResult{
		Branch:      sr.Branch,
		Synced:      sr.Synced,
		Skipped:     sr.Skipped,
		SkipReason:  sr.SkipReason,
		Failed:      sr.Failed,
		FailureHint: sr.FailureHint,
		Pushed:      sr.Pushed,
		PushSkipped: sr.PushSkipped,
		PushFailed:  sr.PushFailed,
		PushError:   sr.PushError,
	}
}
