package operations

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

const errWorktreeNotFound = "worktree not found for task %q"

// AddResult holds the outcome of adding a worktree.
type AddResult struct {
	Task      string
	Branch    string
	Path      string
	Copied    []string
	Source    string
}

// AddWorktree creates a new worktree with the given task and prefix.
func AddWorktree(r git.Runner, cfg *config.Config, task, prefix, source string) (*AddResult, error) {
	repoRoot, err := git.RepoRoot(r)
	if err != nil {
		return nil, err
	}

	if source == "" {
		source = cfg.DefaultSource
	}

	branch := resolver.BranchName(prefix, task)
	wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
	wtPath := resolver.WorktreePath(wtDir, branch)

	if git.BranchExists(r, branch) {
		return nil, fmt.Errorf("branch %q already exists", branch)
	}
	if _, err := os.Stat(wtPath); err == nil {
		return nil, fmt.Errorf("worktree path already exists: %s", wtPath)
	}

	if err := git.AddWorktree(r, wtPath, branch, source); err != nil {
		return nil, err
	}

	copied, err := fileutil.CopyEntries(repoRoot, wtPath, cfg.CopyFiles)
	if err != nil {
		return nil, fmt.Errorf("worktree created but failed to copy files: %w\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, wtPath, task)
	}

	return &AddResult{
		Task:   task,
		Branch: branch,
		Path:   wtPath,
		Copied: copied,
		Source: source,
	}, nil
}

// RemoveResult holds the outcome of removing a worktree.
type RemoveResult struct {
	Path           string
	Branch         string
	BranchDeleted  bool
	BranchError    error
}

// RemoveWorktree removes a worktree and optionally deletes its branch.
func RemoveWorktree(r git.Runner, task string, force, keepBranch bool) (*RemoveResult, error) {
	wt, err := findWorktree(r, task)
	if err != nil {
		return nil, err
	}

	if err := git.RemoveWorktree(r, wt.Path, force); err != nil {
		return nil, err
	}

	result := &RemoveResult{
		Path:   wt.Path,
		Branch: wt.Branch,
	}

	if !keepBranch {
		if err := git.DeleteBranch(r, wt.Branch, true); err != nil {
			result.BranchError = err
		} else {
			result.BranchDeleted = true
		}
	}

	return result, nil
}

// MergeResult holds the outcome of a merge operation.
type MergeResult struct {
	SourceBranch   string
	TargetLabel    string
	Deleted        bool
	DeleteError    error
}

// MergeWorktree merges a source worktree into a target (main or another worktree).
func MergeWorktree(r git.Runner, cfg *config.Config, sourceTask, intoTask string, noFF, shouldDelete bool) (*MergeResult, error) {
	repoRoot, err := git.RepoRoot(r)
	if err != nil {
		return nil, err
	}

	worktrees, err := listWorktreeInfos(r)
	if err != nil {
		return nil, err
	}

	prefixes := resolver.AllPrefixes()

	source, found := resolver.FindBranchForTask(sourceTask, worktrees, prefixes)
	if !found {
		return nil, fmt.Errorf(errWorktreeNotFound, sourceTask)
	}

	var targetDir, targetLabel string
	mergingToMain := intoTask == ""

	if mergingToMain {
		targetDir = repoRoot
		targetLabel = cfg.DefaultSource
	} else {
		target, found := resolver.FindBranchForTask(intoTask, worktrees, prefixes)
		if !found {
			return nil, fmt.Errorf(errWorktreeNotFound, intoTask)
		}
		targetDir = target.Path
		targetLabel = target.Branch
	}

	// Check for dirty state.
	srcDirty, err := git.IsDirty(r, source.Path)
	if err != nil {
		return nil, err
	}
	if srcDirty {
		return nil, fmt.Errorf("source worktree %q has uncommitted changes", sourceTask)
	}

	tgtDirty, err := git.IsDirty(r, targetDir)
	if err != nil {
		return nil, err
	}
	if tgtDirty {
		return nil, fmt.Errorf("target %q has uncommitted changes", targetLabel)
	}

	if err := git.Merge(r, targetDir, source.Branch, noFF); err != nil {
		return nil, fmt.Errorf("merge failed: %w\nTo resolve conflicts: cd %s", err, targetDir)
	}

	result := &MergeResult{
		SourceBranch: source.Branch,
		TargetLabel:  targetLabel,
	}

	if shouldDelete {
		result.Deleted, result.DeleteError = deleteMergedWorktree(r, source)
	}

	return result, nil
}

// deleteMergedWorktree removes the source worktree and its branch after a successful merge.
func deleteMergedWorktree(r git.Runner, source resolver.WorktreeInfo) (bool, error) {
	if err := git.RemoveWorktree(r, source.Path, false); err != nil {
		return false, fmt.Errorf("remove worktree: %w", err)
	}
	if err := git.DeleteBranch(r, source.Branch, true); err != nil {
		return false, fmt.Errorf("delete branch: %w", err)
	}
	return true, nil
}

// SyncWorktree syncs a single worktree with the main branch.
func SyncWorktree(r git.Runner, task, mainBranch string, useMerge bool) error {
	wt, err := findWorktree(r, task)
	if err != nil {
		return err
	}

	dirty, err := git.IsDirty(r, wt.Path)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("worktree %q has uncommitted changes", task)
	}

	if useMerge {
		return git.Merge(r, wt.Path, mainBranch, false)
	}
	if err := git.Rebase(r, wt.Path, mainBranch); err != nil {
		_ = git.AbortRebase(r, wt.Path)
		return err
	}
	return nil
}

func findWorktree(r git.Runner, task string) (resolver.WorktreeInfo, error) {
	worktrees, err := listWorktreeInfos(r)
	if err != nil {
		return resolver.WorktreeInfo{}, err
	}

	wt, found := resolver.FindBranchForTask(task, worktrees, resolver.AllPrefixes())
	if !found {
		return resolver.WorktreeInfo{}, fmt.Errorf(errWorktreeNotFound, task)
	}
	return wt, nil
}

func listWorktreeInfos(r git.Runner) ([]resolver.WorktreeInfo, error) {
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil, err
	}

	worktrees := make([]resolver.WorktreeInfo, len(entries))
	for i, e := range entries {
		worktrees[i] = resolver.WorktreeInfo{
			Path:   e.Path,
			Branch: e.Branch,
		}
	}
	return worktrees, nil
}
