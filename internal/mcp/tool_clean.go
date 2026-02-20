package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerCleanTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("clean",
		mcp.WithDescription("Clean up stale worktree references, merged branches, or stale worktrees"),
		mcp.WithString("mode",
			mcp.Description("Clean mode: prune (stale refs), merged (merged branches), or stale (inactive worktrees)"),
			mcp.Required(),
			mcp.Enum("prune", "merged", "stale"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Preview what would be cleaned without making changes"),
		),
		mcp.WithNumber("stale_days",
			mcp.Description("Number of days to consider a worktree stale (default: 14, used with mode=stale)"),
		),
	)
	s.AddTool(tool, handleClean(hctx))
}

func handleClean(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mode := req.GetString("mode", "")
		if mode == "" {
			return mcp.NewToolResultError("mode is required"), nil
		}

		dryRun := req.GetBool("dry_run", false)
		staleDays := req.GetInt("stale_days", 14)

		r := hctx.Runner

		switch mode {
		case "prune":
			return cleanPrune(r, dryRun)
		case "merged":
			return cleanMerged(r, hctx, dryRun)
		case "stale":
			return cleanStale(r, hctx, dryRun, staleDays)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("invalid mode %q; use prune, merged, or stale", mode)), nil
		}
	}
}

func cleanPrune(r git.Runner, dryRun bool) (*mcp.CallToolResult, error) {
	output, err := git.Prune(r, dryRun)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return marshalResult(cleanResult{
		Mode:    "prune",
		DryRun:  dryRun,
		Removed: make([]cleanedItem, 0),
		Output:  output,
	})
}

func cleanMerged(r git.Runner, hctx *HandlerContext, dryRun bool) (*mcp.CallToolResult, error) {
	mainBranch, err := resolveMainBranch(r, hctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Fetch latest (non-fatal)
	mergeRef := mainBranch
	if err := git.Fetch(r, "origin"); err == nil {
		mergeRef = "origin/" + mainBranch
	}

	candidates, err := findMergedCandidates(r, mergeRef, mainBranch)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(candidates) == 0 || dryRun {
		items := make([]cleanedItem, len(candidates))
		for i, c := range candidates {
			items[i] = cleanedItem{Branch: c.branch, Path: c.path}
		}
		return marshalResult(cleanResult{
			Mode:    "merged",
			DryRun:  dryRun,
			Removed: items,
		})
	}

	// Force mode: no confirmation prompts
	removed := removeCandidates(r, candidates)
	return marshalResult(cleanResult{
		Mode:    "merged",
		DryRun:  false,
		Removed: removed,
	})
}

func cleanStale(r git.Runner, hctx *HandlerContext, dryRun bool, staleDays int) (*mcp.CallToolResult, error) {
	mainBranch, err := resolveMainBranch(r, hctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	threshold := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)

	var candidates []mergedCandidate
	for _, e := range git.FilterEntries(entries, mainBranch) {
		ct, err := git.LastCommitTime(r, e.Branch)
		if err != nil {
			continue
		}
		if ct.Before(threshold) {
			candidates = append(candidates, mergedCandidate{path: e.Path, branch: e.Branch})
		}
	}

	if len(candidates) == 0 || dryRun {
		items := make([]cleanedItem, len(candidates))
		for i, c := range candidates {
			items[i] = cleanedItem{Branch: c.branch, Path: c.path}
		}
		return marshalResult(cleanResult{
			Mode:    "stale",
			DryRun:  dryRun,
			Removed: items,
		})
	}

	removed := removeCandidates(r, candidates)
	return marshalResult(cleanResult{
		Mode:    "stale",
		DryRun:  false,
		Removed: removed,
	})
}

// mergedCandidate holds a branch/path pair for removal.
type mergedCandidate struct {
	path   string
	branch string
}

// findMergedCandidates finds worktrees whose branches are merged into mergeRef.
func findMergedCandidates(r git.Runner, mergeRef, mainBranch string) ([]mergedCandidate, error) {
	mergedList, err := git.MergedBranches(r, mergeRef)
	if err != nil {
		return nil, fmt.Errorf("failed to list merged branches: %w", err)
	}

	mergedSet := make(map[string]bool, len(mergedList))
	for _, b := range mergedList {
		mergedSet[b] = true
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	var candidates []mergedCandidate
	for _, e := range git.FilterEntries(entries, mainBranch) {
		if mergedSet[e.Branch] {
			candidates = append(candidates, mergedCandidate{path: e.Path, branch: e.Branch})
			continue
		}

		// Fallback: squash-merge detection
		squashed, err := git.IsSquashMerged(r, mergeRef, e.Branch)
		if err != nil {
			continue
		}
		if squashed {
			candidates = append(candidates, mergedCandidate{path: e.Path, branch: e.Branch})
		}
	}
	return candidates, nil
}

// removeCandidates removes worktrees and branches, returning successfully cleaned items.
func removeCandidates(r git.Runner, candidates []mergedCandidate) []cleanedItem {
	var removed []cleanedItem
	for _, c := range candidates {
		if err := git.RemoveWorktree(r, c.path, false); err != nil {
			continue
		}
		if err := git.DeleteBranch(r, c.branch, true); err != nil {
			// Worktree removed but branch not — still report
			removed = append(removed, cleanedItem{Branch: c.branch, Path: c.path})
			continue
		}
		removed = append(removed, cleanedItem{Branch: c.branch, Path: c.path})
	}
	return removed
}
