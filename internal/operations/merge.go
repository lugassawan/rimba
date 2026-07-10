package operations

import (
	"context"
	"fmt"
	"sync"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

// MergeParams holds the inputs for a merge operation.
type MergeParams struct {
	SourceTask    string
	SourceService string
	// Source, when set, skips re-resolving the source worktree from
	// SourceTask/SourceService — for callers that already resolved it (e.g. for a guard check).
	Source      *resolver.WorktreeInfo
	IntoTask    string // empty = merge into main
	IntoService string
	RepoRoot    string
	MainBranch  string
	NoFF        bool
	Keep        bool
	Delete      bool
	DryRun      bool
}

// MergeResult holds the outcome of a merge operation.
type MergeResult struct {
	SourceBranch    string
	SourcePath      string
	TargetLabel     string
	MergingToMain   bool
	SourceRemoved   bool  // true only if both worktree removed and branch deleted
	WorktreeRemoved bool  // true if worktree was removed (branch may still exist)
	SourcePrunable  bool  // true if the source's admin entry was prunable (#374) — informs the cleanup-failure hint
	RemoveError     error // non-nil if cleanup failed
	RemoteDeleted   bool  // true if the remote branch was deleted (parity with clean --merged, #231)
	RemoteError     error // non-nil if remote-branch deletion was attempted and failed
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
	source, targetDir, targetLabel, mergingToMain, err := resolveMergeEndpoints(ctx, r, params)
	if err != nil {
		return MergeResult{}, err
	}

	plan := &Plan{DryRun: params.DryRun}
	result := MergeResult{
		SourceBranch:   source.Branch,
		SourcePath:     source.Path,
		TargetLabel:    targetLabel,
		MergingToMain:  mergingToMain,
		SourcePrunable: source.Prunable,
		Plan:           plan,
	}

	// Concurrent dirty checks (always run — read-only pre-flight)
	progress.Notify(onProgress, "Checking for uncommitted changes...")
	if err := checkDirty(ctx, r, dirtyCheckArgs{
		sourcePath:     source.Path,
		targetDir:      targetDir,
		sourceTask:     params.SourceTask,
		intoTask:       params.IntoTask,
		targetLabel:    targetLabel,
		mergingToMain:  mergingToMain,
		sourcePrunable: source.Prunable,
	}); err != nil {
		return result, err
	}

	// Execute merge
	progress.Notify(onProgress, "Merging...")
	mergeDesc := fmt.Sprintf("merge %s into %s", source.Branch, targetLabel)
	if err := plan.Do(mergeDesc, func() error {
		return git.Merge(ctx, r, targetDir, source.Branch, params.NoFF)
	}); err != nil {
		return result, abortFailedMerge(ctx, r, targetDir, targetLabel, source.Branch, err)
	}

	// Auto-cleanup
	shouldDelete := (mergingToMain && !params.Keep) || (!mergingToMain && params.Delete)
	if shouldDelete {
		progress.Notify(onProgress, "Removing worktree...")
		var wtRemoved, brDeleted bool
		rmErr := plan.Do("remove worktree: "+source.Path, func() error {
			var err error
			wtRemoved, brDeleted, err = removeAndCleanup(ctx, r, source.Path, source.Branch, false, source.Prunable)
			return err
		})
		result.WorktreeRemoved = wtRemoved
		result.SourceRemoved = wtRemoved && brDeleted
		if rmErr != nil {
			result.RemoveError = rmErr
		}

		// Delete the merged remote branch (best-effort, parity with clean --merged, #231).
		// Gated on wtRemoved (not brDeleted) so that a partial failure — branch deleted but
		// worktree still on disk — defers remote cleanup to a later `rimba clean --merged` run.
		deleteMergedRemote(ctx, r, plan, source.Branch, &result, params.DryRun, wtRemoved)
	}

	return result, nil
}

// resolveMergeEndpoints resolves the source and target worktrees for a merge.
// Skips the worktree-list lookup entirely when params.Source is pre-resolved and merging to main.
func resolveMergeEndpoints(ctx context.Context, r git.Runner, params MergeParams) (source resolver.WorktreeInfo, targetDir, targetLabel string, mergingToMain bool, err error) {
	mergingToMain = params.IntoTask == ""

	if params.Source != nil && mergingToMain {
		return *params.Source, params.RepoRoot, params.MainBranch, true, nil
	}

	worktrees, err := ListWorktreeInfos(ctx, r)
	if err != nil {
		return resolver.WorktreeInfo{}, "", "", mergingToMain, err
	}
	prefixes := config.PrefixSetFromContext(ctx).Strip()

	if params.Source != nil {
		source = *params.Source
	} else {
		var found bool
		if source, found = resolver.FindBranchForTask(params.SourceService, params.SourceTask, worktrees, prefixes); !found {
			return resolver.WorktreeInfo{}, "", "", mergingToMain, fmt.Errorf(ErrWorktreeNotFoundFmt, params.SourceTask)
		}
	}

	if mergingToMain {
		return source, params.RepoRoot, params.MainBranch, true, nil
	}

	target, tgtFound := resolver.FindBranchForTask(params.IntoService, params.IntoTask, worktrees, prefixes)
	if !tgtFound {
		return resolver.WorktreeInfo{}, "", "", mergingToMain, fmt.Errorf(ErrWorktreeNotFoundFmt, params.IntoTask)
	}
	return source, target.Path, target.Branch, mergingToMain, nil
}

// deleteMergedRemote deletes the remote tracking branch after a merge cleanup.
// It is a no-op when the remote is absent or when the worktree was not actually
// removed. Failures are captured in result.RemoteError and never abort the merge.
func deleteMergedRemote(ctx context.Context, r git.Runner, plan *Plan, branch string, result *MergeResult, dryRun, wtRemoved bool) {
	if !dryRun && !wtRemoved {
		return
	}
	if !git.RemoteExists(ctx, r, git.DefaultRemote) {
		return
	}
	_ = plan.Do("delete remote branch: "+git.DefaultRemote+"/"+branch, func() error {
		var remoteErr error
		if remoteErr = git.DeleteRemoteBranch(ctx, r, git.DefaultRemote, branch); remoteErr == nil {
			result.RemoteDeleted = true
		}
		result.RemoteError = remoteErr
		return nil // best-effort: never abort the merge
	})
}

// abortFailedMerge attempts to roll back a failed merge in targetDir.
// It checks MERGE_HEAD first so we only abort when a merge is actually in progress.
// If the abort also fails, both errors are surfaced with a manual-cleanup hint.
// If the MERGE_HEAD check itself fails (infrastructure error), a conservative
// manual-cleanup hint is returned rather than assuming no merge is in progress.
func abortFailedMerge(ctx context.Context, r git.Runner, targetDir, targetLabel, sourceBranch string, mergeErr error) error {
	inProgress, checkErr := git.MergeInProgress(ctx, r, targetDir)
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

type dirtyCheckArgs struct {
	sourcePath, targetDir string
	sourceTask, intoTask  string
	targetLabel           string
	mergingToMain         bool
	// sourcePrunable skips the source's dirty check: a prunable source has no
	// live .git, so `git status` against it fails with a raw git error rather
	// than a meaningful dirty/clean answer — there's no index to be dirty.
	sourcePrunable bool
}

// checkDirty checks both source and target for uncommitted changes concurrently.
func checkDirty(ctx context.Context, r git.Runner, args dirtyCheckArgs) error {
	var srcCheck, tgtCheck dirtyResult
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if args.sourcePrunable {
			return
		}
		srcCheck.dirty, srcCheck.err = git.IsDirty(ctx, r, args.sourcePath)
	}()
	go func() {
		defer wg.Done()
		tgtCheck.dirty, tgtCheck.err = git.IsDirty(ctx, r, args.targetDir)
	}()
	wg.Wait()

	if srcCheck.err != nil {
		return srcCheck.err
	}
	if srcCheck.dirty {
		return errhint.WithFix(
			fmt.Errorf("source worktree %q has uncommitted changes", args.sourceTask),
			"Commit or stash changes before merging: cd "+args.sourcePath,
		)
	}
	if tgtCheck.err != nil {
		return tgtCheck.err
	}
	if tgtCheck.dirty {
		label := args.targetLabel
		if !args.mergingToMain {
			label = args.intoTask
		}
		return errhint.WithFix(
			fmt.Errorf("target %q has uncommitted changes", label),
			"Commit or stash changes before merging: cd "+args.targetDir,
		)
	}
	return nil
}
