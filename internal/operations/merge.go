package operations

import (
	"fmt"
	"sync"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// MergeParams holds the inputs for a merge operation.
type MergeParams struct {
	SourceTask string
	IntoTask   string // empty = merge into main
	RepoRoot   string
	MainBranch string
	NoFF       bool
	Keep       bool
	Delete     bool
}

// MergeResult holds the outcome of a merge operation.
type MergeResult struct {
	SourceBranch    string
	SourcePath      string
	TargetLabel     string
	MergingToMain   bool
	SourceRemoved   bool  // true only if both worktree removed and branch deleted
	WorktreeRemoved bool  // true if worktree was removed (branch may still exist)
	RemoveError     error // non-nil if cleanup failed
}

// dirtyResult holds the outcome of an IsDirty check.
type dirtyResult struct {
	dirty bool
	err   error
}

// MergeWorktree merges a source worktree branch into main or another worktree.
// It handles worktree resolution, concurrent dirty checks, merge execution, and cleanup.
func MergeWorktree(r git.Runner, params MergeParams, onProgress ProgressFunc) (MergeResult, error) {
	worktrees, err := ListWorktreeInfos(r)
	if err != nil {
		return MergeResult{}, err
	}

	prefixes := resolver.AllPrefixes()

	// Resolve source
	source, found := resolver.FindBranchForTask(params.SourceTask, worktrees, prefixes)
	if !found {
		return MergeResult{}, fmt.Errorf(ErrWorktreeNotFoundFmt, params.SourceTask)
	}

	// Resolve target
	var targetDir, targetLabel string
	mergingToMain := params.IntoTask == ""

	if mergingToMain {
		targetDir = params.RepoRoot
		targetLabel = params.MainBranch
	} else {
		target, tgtFound := resolver.FindBranchForTask(params.IntoTask, worktrees, prefixes)
		if !tgtFound {
			return MergeResult{}, fmt.Errorf(ErrWorktreeNotFoundFmt, params.IntoTask)
		}
		targetDir = target.Path
		targetLabel = target.Branch
	}

	result := MergeResult{
		SourceBranch:  source.Branch,
		SourcePath:    source.Path,
		TargetLabel:   targetLabel,
		MergingToMain: mergingToMain,
	}

	// Concurrent dirty checks
	notify(onProgress, "Checking for uncommitted changes...")
	if err := checkDirty(r, source.Path, targetDir, params.SourceTask, params.IntoTask, mergingToMain, targetLabel); err != nil {
		return result, err
	}

	// Execute merge
	notify(onProgress, "Merging...")
	if err := git.Merge(r, targetDir, source.Branch, params.NoFF); err != nil {
		return result, fmt.Errorf("merge failed: %w\nTo resolve conflicts: cd %s", err, targetDir)
	}

	// Auto-cleanup
	shouldDelete := (mergingToMain && !params.Keep) || (!mergingToMain && params.Delete)
	if shouldDelete {
		notify(onProgress, "Removing worktree...")
		wtRemoved, brDeleted, rmErr := removeAndCleanup(r, source.Path, source.Branch)
		result.WorktreeRemoved = wtRemoved
		result.SourceRemoved = wtRemoved && brDeleted
		if rmErr != nil {
			result.RemoveError = rmErr
		}
	}

	return result, nil
}

// checkDirty checks both source and target for uncommitted changes concurrently.
func checkDirty(r git.Runner, sourcePath, targetDir, sourceTask, intoTask string, mergingToMain bool, targetLabel string) error {
	var srcCheck, tgtCheck dirtyResult
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
		return srcCheck.err
	}
	if srcCheck.dirty {
		return fmt.Errorf("source worktree %q has uncommitted changes\nCommit or stash changes before merging: cd %s", sourceTask, sourcePath)
	}
	if tgtCheck.err != nil {
		return tgtCheck.err
	}
	if tgtCheck.dirty {
		label := targetLabel
		if !mergingToMain {
			label = intoTask
		}
		return fmt.Errorf("target %q has uncommitted changes\nCommit or stash changes before merging: cd %s", label, targetDir)
	}
	return nil
}
