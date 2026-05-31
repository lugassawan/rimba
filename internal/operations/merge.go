package operations

import (
	"context"
	"fmt"
	"sync"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

// MergeParams holds the inputs for a merge operation.
type MergeParams struct {
	SourceTask    string
	SourceService string
	IntoTask      string // empty = merge into main
	IntoService   string
	RepoRoot      string
	MainBranch    string
	NoFF          bool
	Keep          bool
	Delete        bool
	DryRun        bool
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
	Plan            *Plan // always non-nil on a successful return; records steps that were (or would be) executed
}

// dirtyResult holds the outcome of an IsDirty check.
type dirtyResult struct {
	dirty bool
	err   error
}

// MergeWorktree merges a source worktree branch into main or another worktree.
// It handles worktree resolution, concurrent dirty checks, merge execution, and cleanup.
func MergeWorktree(ctx context.Context, r git.Runner, params MergeParams, onProgress progress.Func) (MergeResult, error) {
	worktrees, err := ListWorktreeInfos(r)
	if err != nil {
		return MergeResult{}, err
	}

	prefixes := resolver.AllPrefixes()

	// Resolve source
	source, found := resolver.FindBranchForTask(params.SourceService, params.SourceTask, worktrees, prefixes)
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
		target, tgtFound := resolver.FindBranchForTask(params.IntoService, params.IntoTask, worktrees, prefixes)
		if !tgtFound {
			return MergeResult{}, fmt.Errorf(ErrWorktreeNotFoundFmt, params.IntoTask)
		}
		targetDir = target.Path
		targetLabel = target.Branch
	}

	plan := &Plan{DryRun: params.DryRun}
	result := MergeResult{
		SourceBranch:  source.Branch,
		SourcePath:    source.Path,
		TargetLabel:   targetLabel,
		MergingToMain: mergingToMain,
		Plan:          plan,
	}

	// Concurrent dirty checks (always run — read-only pre-flight)
	progress.Notify(onProgress, "Checking for uncommitted changes...")
	if err := checkDirty(r, source.Path, targetDir, params.SourceTask, params.IntoTask, mergingToMain, targetLabel); err != nil {
		return result, err
	}

	// Execute merge
	progress.Notify(onProgress, "Merging...")
	mergeDesc := fmt.Sprintf("merge %s into %s", source.Branch, targetLabel)
	if err := plan.Do(mergeDesc, func() error {
		return git.Merge(ctx, r, targetDir, source.Branch, params.NoFF)
	}); err != nil {
		return result, abortFailedMerge(r, targetDir, targetLabel, source.Branch, err)
	}

	// Auto-cleanup
	shouldDelete := (mergingToMain && !params.Keep) || (!mergingToMain && params.Delete)
	if shouldDelete {
		progress.Notify(onProgress, "Removing worktree...")
		var wtRemoved, brDeleted bool
		rmErr := plan.Do("remove worktree: "+source.Path, func() error {
			var err error
			wtRemoved, brDeleted, err = removeAndCleanup(r, source.Path, source.Branch)
			return err
		})
		result.WorktreeRemoved = wtRemoved
		result.SourceRemoved = wtRemoved && brDeleted
		if rmErr != nil {
			result.RemoveError = rmErr
		}
	}

	return result, nil
}

// abortFailedMerge attempts to roll back a failed merge in targetDir.
// It checks MERGE_HEAD first so we only abort when a merge is actually in progress.
// If the abort also fails, both errors are surfaced with a manual-cleanup hint.
// If the MERGE_HEAD check itself fails (infrastructure error), a conservative
// manual-cleanup hint is returned rather than assuming no merge is in progress.
func abortFailedMerge(r git.Runner, targetDir, targetLabel, sourceBranch string, mergeErr error) error {
	inProgress, checkErr := git.MergeInProgress(r, targetDir)
	if checkErr != nil {
		// Cannot determine merge state — conservative: tell user to clean up manually.
		return errhint.WithFix(
			fmt.Errorf("merge failed: %w", mergeErr),
			"clean up manually: cd "+targetDir+" && git merge --abort",
		)
	}
	if inProgress {
		if abortErr := git.MergeAbort(r, targetDir); abortErr != nil {
			return errhint.WithFix(
				fmt.Errorf("merge failed and rollback failed: %w (rollback: %w)", mergeErr, abortErr),
				"clean up manually: cd "+targetDir+" && git merge --abort",
			)
		}
		return errhint.WithFix(
			fmt.Errorf("merge failed: %w", mergeErr),
			fmt.Sprintf("%s restored to pre-merge state; reconcile %s, then re-run rimba merge", targetLabel, sourceBranch),
		)
	}
	return errhint.WithFix(
		fmt.Errorf("merge failed: %w", mergeErr),
		fmt.Sprintf("target %s unchanged; reconcile %s, then re-run rimba merge", targetLabel, sourceBranch),
	)
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
		return errhint.WithFix(
			fmt.Errorf("source worktree %q has uncommitted changes", sourceTask),
			"Commit or stash changes before merging: cd "+sourcePath,
		)
	}
	if tgtCheck.err != nil {
		return tgtCheck.err
	}
	if tgtCheck.dirty {
		label := targetLabel
		if !mergingToMain {
			label = intoTask
		}
		return errhint.WithFix(
			fmt.Errorf("target %q has uncommitted changes", label),
			"Commit or stash changes before merging: cd "+targetDir,
		)
	}
	return nil
}
