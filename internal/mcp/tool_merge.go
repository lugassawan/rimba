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

func registerMergeTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("merge",
		mcp.WithDescription("Merge a worktree branch into main or another worktree"),
		mcp.WithString("source",
			mcp.Description("Source task to merge"),
			mcp.Required(),
		),
		mcp.WithString("into",
			mcp.Description("Target task to merge into (default: main branch)"),
		),
		mcp.WithBoolean("no_ff",
			mcp.Description("Force a merge commit (no fast-forward)"),
		),
		mcp.WithBoolean("keep",
			mcp.Description("Keep source worktree after merging into main"),
		),
		mcp.WithBoolean("delete",
			mcp.Description("Delete source worktree after merging into another worktree"),
		),
	)
	s.AddTool(tool, handleMerge(hctx))
}

// mergeDirtyCheck holds the dirty-check result for a single path.
type mergeDirtyCheck struct {
	dirty bool
	err   error
}

func handleMerge(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sourceTask := req.GetString("source", "")
		if sourceTask == "" {
			return mcp.NewToolResultError("source is required"), nil
		}

		intoTask := req.GetString("into", "")
		noFF := req.GetBool("no_ff", false)
		keep := req.GetBool("keep", false)
		del := req.GetBool("delete", false)

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		r := hctx.Runner

		worktrees, err := operations.ListWorktreeInfos(r)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		prefixes := resolver.AllPrefixes()

		// Resolve source
		source, found := resolver.FindBranchForTask(sourceTask, worktrees, prefixes)
		if !found {
			return mcp.NewToolResultError("worktree not found for task \"" + sourceTask + "\""), nil
		}

		// Resolve target
		var targetDir, targetLabel string
		mergingToMain := intoTask == ""

		if mergingToMain {
			targetDir = hctx.RepoRoot
			targetLabel = cfg.DefaultSource
		} else {
			target, tgtFound := resolver.FindBranchForTask(intoTask, worktrees, prefixes)
			if !tgtFound {
				return mcp.NewToolResultError("worktree not found for task \"" + intoTask + "\""), nil
			}
			targetDir = target.Path
			targetLabel = target.Branch
		}

		// Check dirty status
		if errMsg := checkMergeDirty(r, source.Path, targetDir, sourceTask, intoTask, mergingToMain, targetLabel); errMsg != "" {
			return mcp.NewToolResultError(errMsg), nil
		}

		// Execute merge
		if mergeErr := git.Merge(r, targetDir, source.Branch, noFF); mergeErr != nil {
			return mcp.NewToolResultError(mergeErr.Error()), nil
		}

		result := mergeResult{
			Source:        source.Branch,
			Into:          targetLabel,
			SourceRemoved: false,
		}

		// Auto-cleanup
		shouldDelete := (mergingToMain && !keep) || (!mergingToMain && del)
		if shouldDelete {
			if rmErr := git.RemoveWorktree(r, source.Path, false); rmErr != nil {
				return marshalResult(result)
			}
			if brErr := git.DeleteBranch(r, source.Branch, true); brErr != nil {
				return marshalResult(result)
			}
			result.SourceRemoved = true
		}

		return marshalResult(result)
	}
}

// checkMergeDirty checks both source and target for uncommitted changes concurrently.
// Returns an error message if either is dirty, or empty string if clean.
func checkMergeDirty(r git.Runner, sourcePath, targetDir, sourceTask, intoTask string, mergingToMain bool, targetLabel string) string {
	var srcCheck, tgtCheck mergeDirtyCheck
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		srcCheck.dirty, srcCheck.err = git.IsDirty(r, sourcePath)
	}()
	go func() {
		defer wg.Done()
		tgtCheck.dirty, tgtCheck.err = git.IsDirty(r, targetDir)
	}()
	wg.Wait()

	if srcCheck.err != nil {
		return srcCheck.err.Error()
	}
	if srcCheck.dirty {
		return "source worktree \"" + sourceTask + "\" has uncommitted changes; commit or stash changes before merging"
	}
	if tgtCheck.err != nil {
		return tgtCheck.err.Error()
	}
	if tgtCheck.dirty {
		label := targetLabel
		if !mergingToMain {
			label = intoTask
		}
		return "target \"" + label + "\" has uncommitted changes; commit or stash changes before merging"
	}
	return ""
}
