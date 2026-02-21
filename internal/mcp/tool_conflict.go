package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerConflictCheckTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("conflict-check",
		mcp.WithDescription("Detect file overlaps between worktree branches that may cause merge conflicts"),
		mcp.WithBoolean("dry_merge",
			mcp.Description("Simulate merges with git merge-tree to detect actual conflicts (requires git 2.38+)"),
		),
	)
	s.AddTool(tool, handleConflictCheck(hctx))
}

func handleConflictCheck(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dryMerge := req.GetBool("dry_merge", false)

		cfg, err := hctx.requireConfig()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		r := hctx.Runner

		worktrees, err := operations.ListWorktreeInfos(r)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		prefixes := resolver.AllPrefixes()
		allTasks := operations.CollectTasks(worktrees, prefixes)
		eligible := operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)

		if len(eligible) == 0 {
			return marshalResult(conflictCheckData{
				Overlaps: make([]overlapItem, 0),
			})
		}

		diffs, err := conflict.CollectDiffs(r, cfg.DefaultSource, eligible)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result := conflict.DetectOverlaps(diffs)

		data := conflictCheckData{
			Overlaps:      processOverlaps(result),
			TotalFiles:    result.TotalFiles,
			TotalBranches: result.TotalBranches,
		}

		if dryMerge {
			dryResults, err := conflict.DryMergeAll(r, eligible)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data.DryMerges = processDryMerges(dryResults)
		}

		return marshalResult(data)
	}
}

// processOverlaps converts conflict detection results to overlap items.
func processOverlaps(result *conflict.CheckResult) []overlapItem {
	overlaps := make([]overlapItem, 0, len(result.Overlaps))
	for _, o := range result.Overlaps {
		overlaps = append(overlaps, overlapItem{
			File:     o.File,
			Branches: o.Branches,
			Severity: string(o.Severity),
		})
	}
	return overlaps
}

// processDryMerges converts dry merge results to dry merge items.
func processDryMerges(dryResults []conflict.DryMergeResult) []dryMergeItem {
	merges := make([]dryMergeItem, len(dryResults))
	for i, dr := range dryResults {
		merges[i] = dryMergeItem{
			Branch1:       dr.Branch1,
			Branch2:       dr.Branch2,
			HasConflicts:  dr.HasConflicts,
			ConflictFiles: dr.ConflictFiles,
		}
	}
	return merges
}
