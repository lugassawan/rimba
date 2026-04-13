package operations

import (
	"fmt"
	"os"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

// AddParams holds the inputs for creating a new worktree.
type AddParams struct {
	Task          string
	Service       string
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
	Concurrency   int      // max parallel module installs; 0 = Manager default
}

// AddResult holds the outcome of creating a worktree.
type AddResult struct {
	Task        string
	Service     string
	Branch      string
	Path        string
	Source      string
	Copied      []string
	Skipped     []string // copy_files entries not found
	DepsResults []deps.InstallResult
	HookResults []deps.HookResult
}

// AddWorktree creates a new worktree, copies files, installs deps, and runs hooks.
func AddWorktree(r git.Runner, params AddParams, onProgress progress.Func) (AddResult, error) {
	branch := resolver.FullBranchName(params.Service, params.Prefix, params.Task)
	wtPath := resolver.WorktreePath(params.WorktreeDir, branch)

	result := AddResult{
		Task:    params.Task,
		Service: params.Service,
		Branch:  branch,
		Path:    wtPath,
		Source:  params.Source,
	}

	// Validate
	if git.BranchExists(r, branch) {
		return result, errhint.WithFix(
			fmt.Errorf("branch %q already exists", branch),
			"run 'rimba list' to see existing tasks, or use a different task name",
		)
	}
	if _, err := os.Stat(wtPath); err == nil {
		return result, errhint.WithFix(
			fmt.Errorf("worktree path already exists: %s", wtPath),
			"run 'rimba list' to see existing tasks, or use a different task name",
		)
	}

	// Create worktree
	progress.Notify(onProgress, "Creating worktree...")
	if err := git.AddWorktree(r, wtPath, branch, params.Source); err != nil {
		return result, err
	}

	// Post-create setup: copy files, deps, hooks
	pcResult, err := PostCreateSetup(r, PostCreateParams{
		RepoRoot:      params.RepoRoot,
		WtPath:        wtPath,
		Task:          params.Task,
		Service:       params.Service,
		CopyFiles:     params.CopyFiles,
		SkipDeps:      params.SkipDeps,
		AutoDetect:    params.AutoDetect,
		ConfigModules: params.ConfigModules,
		SkipHooks:     params.SkipHooks,
		PostCreate:    params.PostCreate,
		Concurrency:   params.Concurrency,
	}, onProgress)
	if err != nil {
		return result, err
	}
	result.Copied = pcResult.Copied
	result.Skipped = pcResult.Skipped
	result.DepsResults = pcResult.DepsResults
	result.HookResults = pcResult.HookResults

	return result, nil
}
