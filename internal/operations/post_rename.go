package operations

import (
	"context"
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
)

// PostRenameParams holds the inputs for the post-rename setup sequence.
type PostRenameParams struct {
	WtPath        string
	Service       string
	SkipDeps      bool
	AutoDetect    bool
	ConfigModules []config.ModuleConfig
	SkipHooks     bool
	PostRename    []string
	Concurrency   int
}

// PostRenameResult holds the outcome of the post-rename setup sequence.
type PostRenameResult struct {
	DepsResults []deps.InstallResult
	HookResults []deps.HookResult
}

// PostRenameSetup runs the post-rename sequence: refresh deps and run hooks.
func PostRenameSetup(ctx context.Context, r git.Runner, params PostRenameParams, onProgress progress.Func) (PostRenameResult, error) {
	var result PostRenameResult

	if !params.SkipDeps {
		progress.Notify(onProgress, "Refreshing dependencies...")
		wtEntries, err := git.ListWorktrees(ctx, r)
		if err != nil {
			return result, fmt.Errorf("failed to list worktrees for dependency refresh: %w", err)
		}
		dp := DepsParams{
			WtPath:        params.WtPath,
			Service:       params.Service,
			AutoDetect:    params.AutoDetect,
			ConfigModules: params.ConfigModules,
			Entries:       wtEntries,
			Concurrency:   params.Concurrency,
		}
		result.DepsResults = InstallDeps(ctx, r, dp, onProgress)
	}

	if !params.SkipHooks && len(params.PostRename) > 0 {
		progress.Notify(onProgress, "Running post-rename hooks...")
		result.HookResults = RunPostCreateHooks(ctx, params.WtPath, params.PostRename, nil, onProgress)
	}

	return result, nil
}
