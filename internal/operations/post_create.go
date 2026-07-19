package operations

import (
	"context"
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/observability"
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
	Copied          []string
	Skipped         []string
	SkippedSymlinks []string
	DepsResults     []deps.InstallResult
	HookResults     []deps.HookResult
}

// PostCreateSetup runs the post-create sequence: copy files, install deps, run hooks.
// This is used after creating a worktree via git.AddWorktree, git.AddWorktreeFromBranch, etc.
func PostCreateSetup(ctx context.Context, r git.Runner, params PostCreateParams, onProgress progress.Func) (PostCreateResult, error) {
	var result PostCreateResult
	rec := observability.FromContext(ctx)

	// Copy files
	progress.Notify(onProgress, "Copying files...")
	stop := rec.StartSpan("copy")
	copied, skippedSymlinks, err := fileutil.CopyEntries(params.RepoRoot, params.WtPath, params.CopyFiles)
	stop()
	if err != nil {
		return result, errhint.WithFix(
			fmt.Errorf("failed to copy files: %w\nTo retry, manually copy files to: %s", err, params.WtPath),
			"rimba remove "+params.Task,
		)
	}
	result.Copied = copied
	result.Skipped = fileutil.SkippedEntries(params.CopyFiles, copied)
	result.SkippedSymlinks = skippedSymlinks

	// Dependencies
	if !params.SkipDeps {
		stop := rec.StartSpan("deps")
		progress.Notify(onProgress, "Installing dependencies...")
		wtEntries, err := git.ListWorktrees(ctx, r)
		if err != nil {
			stop()
			return result, errhint.WithFix(
				fmt.Errorf("failed to list worktrees for dependency setup: %w", err),
				"rimba remove "+params.Task,
			)
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
			result.DepsResults = InstallDepsPreferSource(ctx, r, params.SourcePath, dp, onProgress)
		} else {
			result.DepsResults = InstallDeps(ctx, r, dp, onProgress)
		}
		stop()
	}

	// Post-create hooks
	if !params.SkipHooks && len(params.PostCreate) > 0 {
		stop := rec.StartSpan("hooks")
		progress.Notify(onProgress, "Running hooks...")
		result.HookResults = RunPostCreateHooks(ctx, params.WtPath, params.PostCreate, onProgress)
		stop()
	}

	return result, nil
}
