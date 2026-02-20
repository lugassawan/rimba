package mcp

import (
	"context"
	"sync"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
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

func handleSync(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		task := req.GetString("task", "")
		all := req.GetBool("all", false)
		useMerge := req.GetBool("merge", false)
		includeInherited := req.GetBool("include_inherited", false)
		noPush := req.GetBool("no_push", false)
		push := !noPush

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		if !all && task == "" {
			return mcp.NewToolResultError("provide a task name or set all=true to sync all worktrees"), nil
		}

		r := hctx.Runner

		// Fetch (non-fatal)
		_ = git.Fetch(r, "origin")

		worktrees, err := operations.ListWorktreeInfos(r)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		prefixes := resolver.AllPrefixes()

		if !all {
			return syncSingle(r, task, worktrees, prefixes, cfg.DefaultSource, useMerge, push)
		}
		return syncMultiple(r, worktrees, prefixes, cfg.DefaultSource, useMerge, includeInherited, push)
	}
}

func syncSingle(r git.Runner, task string, worktrees []resolver.WorktreeInfo, prefixes []string, mainBranch string, useMerge, push bool) (*mcp.CallToolResult, error) {
	wt, found := resolver.FindBranchForTask(task, worktrees, prefixes)
	if !found {
		return mcp.NewToolResultError("worktree not found for task \"" + task + "\""), nil
	}

	sr := operations.SyncWorktree(r, mainBranch, wt, useMerge, push)

	results := []syncWorktreeResult{convertSyncResult(sr)}

	return marshalResult(syncResult{Results: results})
}

func syncMultiple(r git.Runner, worktrees []resolver.WorktreeInfo, prefixes []string, mainBranch string, useMerge, includeInherited, push bool) (*mcp.CallToolResult, error) {
	allTasks := operations.CollectTasks(worktrees, prefixes)
	eligible := operations.FilterEligible(worktrees, prefixes, mainBranch, allTasks, includeInherited)

	results := make([]syncWorktreeResult, len(eligible))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	for i, wt := range eligible {
		wg.Add(1)
		go func(idx int, wt resolver.WorktreeInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sr := operations.SyncWorktree(r, mainBranch, wt, useMerge, push)
			results[idx] = convertSyncResult(sr)
		}(i, wt)
	}
	wg.Wait()

	return marshalResult(syncResult{Results: results})
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
