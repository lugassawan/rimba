package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerSyncTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("sync",
		mcp.WithDescription("Sync worktree(s) with the main branch via rebase or merge, then push (supports 'service/task' for monorepo)"),
		mcp.WithString("task",
			mcp.Description("Single task to sync (e.g. 'my-task' or 'auth-api/my-task' for monorepo)"),
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
	s.AddTool(tool, withRecorder(hctx, "sync", handleSync(hctx)))
}

// syncOpts bundles shared sync configuration derived from a single request.
type syncOpts struct {
	mainBranch   string
	useMerge     bool
	push         bool
	fetchWarning string
	ps           *resolver.PrefixSet
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
			return errorResult(cfgErr), nil
		}

		if !all && task == "" {
			return errorResult(errhint.WithFix(errors.New("provide a task name or set all=true to sync all worktrees"),
				"pass a task name, or set all=true to sync every worktree")), nil
		}

		var service string
		if task != "" {
			service, task = operations.ResolveTaskInput(task, hctx.RepoRoot, hctx.PrefixSet())
		}

		r := hctx.Runner

		// Fetch (non-fatal; cancellation propagated as a tool error)
		fetchWarning, fetchErr := mcpFetchNonFatal(ctx, r, git.FetchArgs{})
		if fetchErr != nil {
			return errorResult(fetchErr), nil
		}

		worktrees, err := operations.ListWorktreeInfos(ctx, r)
		if err != nil {
			return errorResult(err), nil
		}

		ps := cfg.PrefixSet()
		prefixes := ps.Strip()
		opts := syncOpts{
			mainBranch:   cfg.DefaultSource,
			useMerge:     useMerge,
			push:         !noPush,
			fetchWarning: fetchWarning,
			ps:           ps,
		}

		if !all {
			return syncSingle(ctx, r, service, task, worktrees, prefixes, opts)
		}
		return syncMultiple(ctx, r, worktrees, prefixes, opts, includeInherited)
	}
}

func syncSingle(ctx context.Context, r git.Runner, service, task string, worktrees []resolver.WorktreeInfo, prefixes []string, opts syncOpts) (*mcp.CallToolResult, error) {
	wt, found := resolver.FindBranchForTask(service, task, worktrees, prefixes)
	if !found {
		return errorResult(errhint.WithFix(
			fmt.Errorf("worktree not found for task %q", task),
			"run the list tool to see available tasks",
		)), nil
	}

	if err := operations.GuardKnownPrefix(opts.ps, wt.Branch, opts.mainBranch, false); err != nil {
		return errorResult(err), nil
	}

	sr := operations.SyncWorktree(ctx, r, opts.mainBranch, wt, opts.useMerge, opts.push)

	results := []syncWorktreeResult{convertSyncResult(sr)}

	return marshalResult(syncResult{FetchWarning: opts.fetchWarning, Results: results})
}

func syncMultiple(ctx context.Context, r git.Runner, worktrees []resolver.WorktreeInfo, prefixes []string, opts syncOpts, includeInherited bool) (*mcp.CallToolResult, error) {
	allTasks := operations.CollectTasks(worktrees, prefixes)
	eligible := operations.FilterEligible(worktrees, prefixes, opts.mainBranch, allTasks, includeInherited)

	// No per-item timeout here — sync operations (fetch/rebase) are long-running by design.
	results := parallel.Collect(ctx, len(eligible), 4, func(ctx context.Context, i int) syncWorktreeResult {
		return convertSyncResult(operations.SyncWorktree(ctx, r, opts.mainBranch, eligible[i], opts.useMerge, opts.push))
	})

	return marshalResult(syncResult{FetchWarning: opts.fetchWarning, Results: results})
}

// mcpFetchNonFatal fetches from git.DefaultRemote ("origin") and returns a
// warning string on connectivity failure. Cancellation is returned as a hard
// error so callers can propagate it.
func mcpFetchNonFatal(ctx context.Context, r git.Runner, args git.FetchArgs) (warning string, err error) {
	if fetchErr := git.Fetch(ctx, r, git.DefaultRemote, args); fetchErr != nil {
		if errors.Is(fetchErr, context.Canceled) || errors.Is(fetchErr, context.DeadlineExceeded) {
			return "", fetchErr
		}
		return "fetch failed (no remote?): continuing with local state", nil
	}
	return "", nil
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
