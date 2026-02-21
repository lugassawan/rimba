package operations

import (
	"fmt"
	"os"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// AddParams holds the inputs for creating a new worktree.
type AddParams struct {
	Task          string
	Prefix        string // e.g. "feature/"
	Source        string // source branch
	RepoRoot      string
	WorktreeDir   string // absolute path to worktree directory
	CopyFiles     []string
	SkipDeps      bool
	AutoDetect    bool
	ConfigModules []config.ModuleConfig
	SkipHooks     bool
	PostCreate    []string // hook commands
}

// AddResult holds the outcome of creating a worktree.
type AddResult struct {
	Task        string
	Branch      string
	Path        string
	Source      string
	Copied      []string
	Skipped     []string // copy_files entries not found
	DepsResults []deps.InstallResult
	HookResults []deps.HookResult
}

// AddWorktree creates a new worktree, copies files, installs deps, and runs hooks.
func AddWorktree(r git.Runner, params AddParams, onProgress ProgressFunc) (AddResult, error) {
	branch := resolver.BranchName(params.Prefix, params.Task)
	wtPath := resolver.WorktreePath(params.WorktreeDir, branch)

	result := AddResult{
		Task:   params.Task,
		Branch: branch,
		Path:   wtPath,
		Source: params.Source,
	}

	// Validate
	if git.BranchExists(r, branch) {
		return result, fmt.Errorf("branch %q already exists", branch)
	}
	if _, err := os.Stat(wtPath); err == nil {
		return result, fmt.Errorf("worktree path already exists: %s", wtPath)
	}

	// Create worktree
	notify(onProgress, "Creating worktree...")
	if err := git.AddWorktree(r, wtPath, branch, params.Source); err != nil {
		return result, err
	}

	// Copy files
	notify(onProgress, "Copying files...")
	copied, err := fileutil.CopyEntries(params.RepoRoot, wtPath, params.CopyFiles)
	if err != nil {
		return result, fmt.Errorf("worktree created but failed to copy files: %w\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, wtPath, params.Task)
	}
	result.Copied = copied
	result.Skipped = fileutil.SkippedEntries(params.CopyFiles, copied)

	// Dependencies
	if !params.SkipDeps {
		notify(onProgress, "Installing dependencies...")
		wtEntries, _ := git.ListWorktrees(r)
		result.DepsResults = InstallDeps(r, wtPath, params.AutoDetect, params.ConfigModules, wtEntries, nil)
	}

	// Post-create hooks
	if !params.SkipHooks && len(params.PostCreate) > 0 {
		notify(onProgress, "Running hooks...")
		result.HookResults = RunPostCreateHooks(wtPath, params.PostCreate, nil)
	}

	return result, nil
}
