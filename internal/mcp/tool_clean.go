package mcp

import (
	"context"
	"fmt"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
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
			return mcpCleanPrune(ctx, r, dryRun)
		case "merged":
			return mcpCleanMerged(ctx, r, hctx, dryRun)
		case "stale":
			return mcpCleanStale(ctx, r, hctx, dryRun, staleDays)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("invalid mode %q; use prune, merged, or stale", mode)), nil
		}
	}
}

func mcpCleanPrune(ctx context.Context, r git.Runner, dryRun bool) (*mcp.CallToolResult, error) {
	output, err := git.Prune(r, dryRun)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var remotePruned []string
	warnings := []string{}
	remotes, err := git.ListRemotes(r)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to list remotes: %v", err))
	} else {
		var failures []git.RemoteFailure
		remotePruned, failures = git.PruneRemotes(ctx, r, remotes, dryRun)
		for _, f := range failures {
			warnings = append(warnings, fmt.Sprintf("failed to prune %s: %v", f.Remote, f.Err))
		}
	}

	return marshalResult(cleanResult{
		Mode:         "prune",
		DryRun:       dryRun,
		Removed:      make([]cleanedItem, 0),
		Output:       output,
		RemotePruned: remotePruned,
		Warnings:     warnings,
	})
}

func mcpCleanMerged(ctx context.Context, r git.Runner, hctx *HandlerContext, dryRun bool) (*mcp.CallToolResult, error) {
	mainBranch, err := operations.ResolveMainBranch(r, configDefault(hctx))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Fetch latest (non-fatal; cancellation propagated as a tool error)
	mergeRef := mainBranch
	if fetchWarn, fetchErr := mcpFetchNonFatal(ctx, r); fetchErr != nil {
		return mcp.NewToolResultError(fetchErr.Error()), nil
	} else if fetchWarn == "" {
		mergeRef = git.DefaultRemote + "/" + mainBranch
	}

	mergedResult, err := operations.FindMergedCandidates(r, mergeRef, mainBranch)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(mergedResult.Candidates) == 0 || dryRun {
		items := make([]cleanedItem, len(mergedResult.Candidates))
		for i, c := range mergedResult.Candidates {
			items[i] = cleanedItem{Branch: c.Branch, Path: c.Path}
		}
		return marshalResult(cleanResult{
			Mode:     "merged",
			DryRun:   dryRun,
			Removed:  items,
			Warnings: mergedResult.Warnings,
		})
	}

	// Force mode: no confirmation prompts. Probe origin once and pass the result
	// directly so RemoveCandidates does not re-issue git remote get-url per candidate.
	originPresent := git.RemoteExists(r, git.DefaultRemote)
	opItems := operations.RemoveCandidates(ctx, r, mergedResult.Candidates, originPresent, nil)
	warnings := mergedResult.Warnings
	items := make([]cleanedItem, len(opItems))
	for i, item := range opItems {
		items[i] = cleanedItem{Branch: item.Branch, Path: item.Path, RemoteDeleted: item.RemoteDeleted}
		if item.RemoteError != nil {
			warnings = append(warnings, fmt.Sprintf("failed to delete remote branch %s/%s: %v", git.DefaultRemote, item.Branch, item.RemoteError))
		}
	}
	return marshalResult(cleanResult{
		Mode:     "merged",
		DryRun:   false,
		Removed:  items,
		Warnings: warnings,
	})
}

func mcpCleanStale(ctx context.Context, r git.Runner, hctx *HandlerContext, dryRun bool, staleDays int) (*mcp.CallToolResult, error) {
	mainBranch, err := operations.ResolveMainBranch(r, configDefault(hctx))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	staleResult, err := operations.FindStaleCandidates(r, mainBranch, staleDays)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(staleResult.Candidates) == 0 || dryRun {
		items := make([]cleanedItem, len(staleResult.Candidates))
		for i, c := range staleResult.Candidates {
			items[i] = cleanedItem{Branch: c.Branch, Path: c.Path}
		}
		return marshalResult(cleanResult{
			Mode:     "stale",
			DryRun:   dryRun,
			Removed:  items,
			Warnings: staleResult.Warnings,
		})
	}

	toRemove := make([]operations.CleanCandidate, len(staleResult.Candidates))
	for i, c := range staleResult.Candidates {
		toRemove[i] = c.CleanCandidate
	}

	opItems := operations.RemoveCandidates(ctx, r, toRemove, false, nil)
	items := make([]cleanedItem, len(opItems))
	for i, item := range opItems {
		items[i] = cleanedItem{Branch: item.Branch, Path: item.Path}
	}
	return marshalResult(cleanResult{
		Mode:     "stale",
		DryRun:   false,
		Removed:  items,
		Warnings: staleResult.Warnings,
	})
}
