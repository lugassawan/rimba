package mcp

import (
	"context"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerStatusTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("status",
		mcp.WithDescription("Show worktree dashboard with summary stats and age info"),
		mcp.WithNumber("stale_days",
			mcp.Description("Number of days after which a worktree is considered stale (default: 14)"),
		),
	)
	s.AddTool(tool, handleStatus(hctx))
}

// statusCollectedEntry holds per-worktree data gathered concurrently for the status tool.
type statusCollectedEntry struct {
	entry      git.WorktreeEntry
	status     resolver.WorktreeStatus
	commitTime time.Time
	hasTime    bool
}

func handleStatus(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		staleDays := req.GetInt("stale_days", 14)
		r := hctx.Runner

		if _, err := hctx.requireConfig(); err != nil {
			return errorResult(err), nil
		}

		mainBranch, err := operations.ResolveMainBranch(ctx, r, configDefault(hctx))
		if err != nil {
			return errorResult(err), nil
		}

		entries, err := git.ListWorktrees(ctx, r)
		if err != nil {
			return errorResult(err), nil
		}

		candidates := git.FilterEntries(entries, mainBranch)

		if len(candidates) == 0 {
			return marshalResult(statusData{
				Summary:   statusSummary{},
				Worktrees: make([]statusItem, 0),
				StaleDays: staleDays,
			})
		}

		results := parallel.Collect(ctx, len(candidates), 8, func(ctx context.Context, i int) statusCollectedEntry {
			itemCtx, cancel := git.WithItemTimeout(ctx)
			defer cancel()
			e := candidates[i]
			st := operations.CollectWorktreeStatus(itemCtx, r, e.Path)
			var ct time.Time
			var hasTime bool
			if t, err := git.LastCommitTime(itemCtx, r, e.Branch); err == nil {
				ct = t
				hasTime = true
			}
			return statusCollectedEntry{entry: e, status: st, commitTime: ct, hasTime: hasTime}
		})

		staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
		prefixes := hctx.PrefixSet().Strip()

		summary, items := buildStatusResult(results, staleThreshold, prefixes)

		return marshalResult(statusData{
			Summary:   summary,
			Worktrees: items,
			StaleDays: staleDays,
		})
	}
}

// buildStatusResult computes summary counters and status items from collected entries.
func buildStatusResult(results []statusCollectedEntry, staleThreshold time.Time, prefixes []string) (statusSummary, []statusItem) {
	var summary statusSummary
	summary.Total = len(results)

	items := make([]statusItem, 0, len(results))
	for _, r := range results {
		if r.status.Dirty {
			summary.Dirty++
		}
		if r.status.Behind > 0 {
			summary.Behind++
		}

		item := buildStatusItem(r, staleThreshold, prefixes)
		if item.Age != nil && item.Age.Stale {
			summary.Stale++
		}

		items = append(items, item)
	}
	return summary, items
}

// buildStatusItem constructs a statusItem from a collected entry.
func buildStatusItem(r statusCollectedEntry, staleThreshold time.Time, prefixes []string) statusItem {
	task, typeName := resolver.TaskAndType(r.entry.Branch, prefixes)

	item := statusItem{
		Task:   task,
		Type:   typeName,
		Branch: r.entry.Branch,
		Status: r.status,
	}

	if r.hasTime {
		item.Age = &statusAge{
			LastCommit: r.commitTime.UTC().Format(time.RFC3339),
			Stale:      r.commitTime.Before(staleThreshold),
		}
	}

	return item
}
