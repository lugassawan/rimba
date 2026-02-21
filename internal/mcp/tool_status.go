package mcp

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
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

		mainBranch, err := operations.ResolveMainBranch(r, configDefault(hctx))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entries, err := git.ListWorktrees(r)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		candidates := git.FilterEntries(entries, mainBranch)

		if len(candidates) == 0 {
			return marshalResult(statusData{
				Summary:   statusSummary{},
				Worktrees: make([]statusItem, 0),
				StaleDays: staleDays,
			})
		}

		results := make([]statusCollectedEntry, len(candidates))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)

		for i, c := range candidates {
			wg.Add(1)
			go func(idx int, e git.WorktreeEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				st := operations.CollectWorktreeStatus(r, e.Path)
				var ct time.Time
				var hasTime bool
				if t, err := git.LastCommitTime(r, e.Branch); err == nil {
					ct = t
					hasTime = true
				}
				results[idx] = statusCollectedEntry{entry: e, status: st, commitTime: ct, hasTime: hasTime}
			}(i, c)
		}
		wg.Wait()

		staleThreshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
		prefixes := resolver.AllPrefixes()

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

			task, matchedPrefix := resolver.TaskFromBranch(r.entry.Branch, prefixes)
			typeName := strings.TrimSuffix(matchedPrefix, "/")

			item := statusItem{
				Task:   task,
				Type:   typeName,
				Branch: r.entry.Branch,
				Status: r.status,
			}

			if r.hasTime {
				stale := r.commitTime.Before(staleThreshold)
				if stale {
					summary.Stale++
				}
				item.Age = &statusAge{
					LastCommit: r.commitTime.UTC().Format(time.RFC3339),
					Stale:      stale,
				}
			}

			items = append(items, item)
		}

		return marshalResult(statusData{
			Summary:   summary,
			Worktrees: items,
			StaleDays: staleDays,
		})
	}
}
