package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/lugassawan/rimba/internal/errhint"
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
		mcp.WithBoolean("force",
			mcp.Description("Force-remove dirty worktrees even if they contain uncommitted changes (used with mode=merged or mode=stale)"),
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
			return errorResult(errhint.WithFix(errors.New("mode is required"),
				"set mode to one of: prune, merged, stale")), nil
		}

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		dryRun := req.GetBool("dry_run", false)
		force := req.GetBool("force", false)
		staleDays := req.GetInt("stale_days", 14)

		r := hctx.Runner

		switch mode {
		case "prune":
			return mcpCleanPrune(ctx, r, dryRun)
		case "merged":
			return mcpCleanMerged(ctx, r, cfg.DefaultSource, dryRun, force)
		case "stale":
			return mcpCleanStale(ctx, r, cfg.DefaultSource, dryRun, staleDays, force)
		default:
			return errorResult(errhint.WithFix(
				fmt.Errorf("invalid mode %q", mode),
				"use mode: prune, merged, or stale",
			)), nil
		}
	}
}

func mcpCleanPrune(ctx context.Context, r git.Runner, dryRun bool) (*mcp.CallToolResult, error) {
	output, err := git.Prune(ctx, r, dryRun)
	if err != nil {
		return errorResult(err), nil
	}

	var remotePruned []string
	warnings := []string{}
	remotes, err := git.ListRemotes(ctx, r)
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

func mcpCleanMerged(ctx context.Context, r git.Runner, defaultSource string, dryRun bool, force bool) (*mcp.CallToolResult, error) {
	mainBranch, err := operations.ResolveMainBranch(ctx, r, defaultSource)
	if err != nil {
		return errorResult(err), nil
	}

	// Fetch latest (non-fatal; cancellation propagated as a tool error)
	mergeRef := mainBranch
	if fetchWarn, fetchErr := mcpFetchNonFatal(ctx, r, git.FetchArgs{Prune: true}); fetchErr != nil {
		return errorResult(fetchErr), nil
	} else if fetchWarn == "" {
		mergeRef = git.DefaultRemote + "/" + mainBranch
	}

	mergedResult, err := operations.FindMergedCandidates(ctx, r, mergeRef, mainBranch)
	if err != nil {
		return errorResult(err), nil
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

	// Probe origin once so RemoveCandidates does not re-issue git remote get-url per candidate.
	originPresent := git.RemoteExists(ctx, r, git.DefaultRemote)
	opItems := operations.RemoveCandidates(ctx, r, mergedResult.Candidates, originPresent, force, nil)
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

func mcpCleanStale(ctx context.Context, r git.Runner, defaultSource string, dryRun bool, staleDays int, force bool) (*mcp.CallToolResult, error) {
	mainBranch, err := operations.ResolveMainBranch(ctx, r, defaultSource)
	if err != nil {
		return errorResult(err), nil
	}

	staleResult, err := operations.FindStaleCandidates(ctx, r, mainBranch, staleDays)
	if err != nil {
		return errorResult(err), nil
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

	opItems := operations.RemoveCandidates(ctx, r, toRemove, false, force, nil)
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
