package operations

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
)

// PostCreateParams holds the inputs for the post-create setup sequence
// (copy files, install deps, run hooks) shared by add, duplicate, and restore.
type PostCreateParams struct {
	RepoRoot      string
	WtPath        string
	Task          string // for error messages
	Service       string // monorepo service name; scopes dep detection to this subdir
	CopyFiles     []string
	SkipDeps      bool
	AutoDetect    bool
	ConfigModules []config.ModuleConfig
	SkipHooks     bool
	PostCreate    []string // hook commands
	SourcePath    string   // if non-empty, prefer copying deps from this worktree
	Concurrency   int      // max parallel module installs; 0 = Manager default
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
func PostCreateSetup(r git.Runner, params PostCreateParams, onProgress progress.Func) (PostCreateResult, error) {
	var result PostCreateResult

	// Copy files
	progress.Notify(onProgress, "Copying files...")
	copied, err := fileutil.CopyEntries(params.RepoRoot, params.WtPath, params.CopyFiles)
	if err != nil {
		return result, fmt.Errorf("failed to copy files: %w\nTo retry, manually copy files to: %s\nTo remove the worktree: rimba remove %s", err, params.WtPath, params.Task)
	}
	result.Copied = copied
	result.Skipped = fileutil.SkippedEntries(params.CopyFiles, copied)

	// Dependencies
	if !params.SkipDeps {
		progress.Notify(onProgress, "Installing dependencies...")
		wtEntries, err := git.ListWorktrees(r)
		if err != nil {
			return result, fmt.Errorf("failed to list worktrees for dependency setup: %w", err)
		}

		dp := DepsParams{
			WtPath:        params.WtPath,
			Service:       params.Service,
			AutoDetect:    params.AutoDetect,
			ConfigModules: params.ConfigModules,
			Entries:       wtEntries,
			Concurrency:   params.Concurrency,
		}
		if params.SourcePath != "" {
			result.DepsResults = InstallDepsPreferSource(r, params.SourcePath, dp, onProgress)
		} else {
			result.DepsResults = InstallDeps(r, dp, onProgress)
		}
	}

	// Post-create hooks
	if !params.SkipHooks && len(params.PostCreate) > 0 {
		progress.Notify(onProgress, "Running hooks...")
		result.HookResults = RunPostCreateHooks(params.WtPath, params.PostCreate, onProgress)
	}

	return result, nil
}
