package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
)

// PostCreateParams holds the inputs for the post-create setup sequence
// (copy files, install deps, run hooks) shared by add, duplicate, and restore.
type PostCreateParams struct {
	RepoRoot      string
	WtPath        string
	Task          string // for error messages
	CopyFiles     []string
	SkipDeps      bool
	AutoDetect    bool
	ConfigModules []config.ModuleConfig
	SkipHooks     bool
	PostCreate    []string // hook commands
	SourcePath    string   // if non-empty, prefer copying deps from this worktree
}

// PostCreateResult holds the outcome of the post-create setup sequence.
type PostCreateResult struct {
	Copied      []string
	Skipped     []string
	DepsResults []deps.InstallResult
	HookResults []deps.HookResult
}

// PostCreateSetup runs the post-create sequence: copy files, install deps, run hooks.
// This is used after creating a worktree via git.AddWorktree, git.AddWorktreeFromBranch, etc.
func PostCreateSetup(r git.Runner, params PostCreateParams, onProgress ProgressFunc) (PostCreateResult, error) {
	var result PostCreateResult

	// Copy files
	notify(onProgress, "Copying files...")
	copied, err := fileutil.CopyEntries(params.RepoRoot, params.WtPath, params.CopyFiles)
	if err != nil {
		return result, fmt.Errorf("failed to copy files: %w\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, params.WtPath, params.Task)
	}
	result.Copied = copied
	result.Skipped = fileutil.SkippedEntries(params.CopyFiles, copied)

	// Dependencies
	if !params.SkipDeps {
		notify(onProgress, "Installing dependencies...")
		wtEntries, err := git.ListWorktrees(r)
		if err != nil {
			return result, fmt.Errorf("failed to list worktrees for dependency setup: %w", err)
		}

		if params.SourcePath != "" {
			result.DepsResults = InstallDepsPreferSource(r, params.WtPath, params.SourcePath, params.AutoDetect, params.ConfigModules, wtEntries, nil)
		} else {
			result.DepsResults = InstallDeps(r, params.WtPath, params.AutoDetect, params.ConfigModules, wtEntries, nil)
		}
	}

	// Post-create hooks
	if !params.SkipHooks && len(params.PostCreate) > 0 {
		notify(onProgress, "Running hooks...")
		result.HookResults = RunPostCreateHooks(params.WtPath, params.PostCreate, nil)
	}

	return result, nil
}
