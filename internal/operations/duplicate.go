package operations

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

const maxDuplicateSuffix = 1000

// DuplicateParams holds the inputs for creating a worktree from another worktree's branch.
type DuplicateParams struct {
	Source        resolver.WorktreeInfo
	SourceTask    string
	As            string
	PrefixSet     *resolver.PrefixSet
	DefaultSource string
	DryRun        bool
	PostCreateOptions
}

// DuplicateResult holds the outcome of duplicating a worktree.
type DuplicateResult struct {
	SourceTask      string
	SourceBranch    string
	Task            string
	Service         string
	Branch          string
	Path            string
	DryRun          bool
	Copied          []string
	Skipped         []string
	SkippedSymlinks []string
	DepsResults     []deps.InstallResult
	HookResults     []deps.HookResult
}

// DuplicateWorktree creates a new worktree from an existing worktree's branch.
func DuplicateWorktree(ctx context.Context, r git.Runner, params DuplicateParams, onProgress progress.Func) (DuplicateResult, error) {
	result, err := planDuplicate(ctx, r, params)
	if err != nil || params.DryRun {
		return result, err
	}
	return createDuplicate(ctx, r, params, result, onProgress)
}

func planDuplicate(ctx context.Context, r git.Runner, params DuplicateParams) (DuplicateResult, error) {
	ps := params.PrefixSet
	if ps == nil {
		ps = resolver.DefaultPrefixSet()
	}
	if err := GuardKnownPrefix(ps, params.Source.Branch, params.DefaultSource, false); err != nil {
		return DuplicateResult{}, err
	}
	if params.Source.Branch == params.DefaultSource {
		return DuplicateResult{}, fmt.Errorf("cannot duplicate the default branch %q; use 'rimba add' instead", params.DefaultSource)
	}

	service, sourceTask, task, prefix, err := duplicateTarget(ctx, r, params, ps)
	if err != nil {
		return DuplicateResult{}, err
	}
	result := duplicateResult(params, service, sourceTask, task, prefix)
	if git.BranchExists(ctx, r, result.Branch) {
		return result, fmt.Errorf("branch %q already exists", result.Branch)
	}
	if _, err := os.Lstat(result.Path); err == nil {
		return result, fmt.Errorf("worktree path already exists: %s", result.Path)
	}
	return result, nil
}

func duplicateTarget(ctx context.Context, r git.Runner, params DuplicateParams, ps *resolver.PrefixSet) (service, sourceTask, task, prefix string, err error) {
	service, inferredTask, prefix := resolver.ServiceFromBranch(params.Source.Branch, ps.Strip())
	if prefix == "" {
		prefix, _ = resolver.PrefixString(resolver.DefaultPrefixType)
	}
	sourceTask = params.SourceTask
	if sourceTask == "" {
		sourceTask = inferredTask
	}
	if params.As == "" {
		task, err = nextDuplicateTask(ctx, r, sourceTask, service, prefix, params.WorktreeDir)
		return service, sourceTask, task, prefix, err
	}

	input := ClassifyTaskInput(params.As, params.RepoRoot, ps)
	if input.Kind == KindUnknownService {
		candidate, _ := resolver.SplitServiceInput(params.As)
		return "", "", "", "", fmt.Errorf("service %q not found; create the service directory first or omit the service prefix", candidate)
	}
	if err := ValidateBranchInput(input.Task, input.Service); err != nil {
		return "", "", "", "", err
	}
	if input.Service != "" {
		service = input.Service
	}
	return service, sourceTask, input.Task, prefix, nil
}

func duplicateResult(params DuplicateParams, service, sourceTask, task, prefix string) DuplicateResult {
	branch := resolver.FullBranchName(service, prefix, task)
	return DuplicateResult{
		SourceTask:   sourceTask,
		SourceBranch: params.Source.Branch,
		Task:         task,
		Service:      service,
		Branch:       branch,
		Path:         resolver.WorktreePath(params.WorktreeDir, branch),
		DryRun:       params.DryRun,
	}
}

func createDuplicate(ctx context.Context, r git.Runner, params DuplicateParams, result DuplicateResult, onProgress progress.Func) (DuplicateResult, error) {
	progress.Notify(onProgress, "Creating worktree...")
	stop := observability.FromContext(ctx).StartSpan("create")
	err := git.AddWorktree(ctx, r, result.Path, result.Branch, result.SourceBranch)
	stop()
	if err != nil {
		return result, err
	}

	postCreate, err := PostCreateSetup(ctx, r, duplicatePostCreateParams(params, result), onProgress)
	if err != nil {
		return result, RollbackFailedCreate(r, RollbackParams{
			WtPath:     result.Path,
			Branch:     result.Branch,
			Task:       result.Task,
			OnProgress: onProgress,
		}, err)
	}
	result.Copied = postCreate.Copied
	result.Skipped = postCreate.Skipped
	result.SkippedSymlinks = postCreate.SkippedSymlinks
	result.DepsResults = postCreate.DepsResults
	result.HookResults = postCreate.HookResults
	return result, nil
}

func duplicatePostCreateParams(params DuplicateParams, result DuplicateResult) PostCreateParams {
	return PostCreateParams{
		RepoRoot:      params.RepoRoot,
		WtPath:        result.Path,
		Task:          result.Task,
		Service:       result.Service,
		NewBranch:     true,
		CopyFiles:     params.CopyFiles,
		SkipDeps:      params.SkipDeps,
		AutoDetect:    params.AutoDetect,
		ConfigModules: params.ConfigModules,
		SkipHooks:     params.SkipHooks,
		PostCreate:    params.PostCreate,
		SourcePath:    params.Source.Path,
		Concurrency:   params.Concurrency,
	}
}

func nextDuplicateTask(ctx context.Context, r git.Runner, task, service, prefix, worktreeDir string) (string, error) {
	for i := 1; i <= maxDuplicateSuffix; i++ {
		candidate := fmt.Sprintf("%s-%d", task, i)
		branch := resolver.FullBranchName(service, prefix, candidate)
		if git.BranchExists(ctx, r, branch) {
			continue
		}
		path := resolver.WorktreePath(worktreeDir, branch)
		_, err := os.Lstat(path)
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("check worktree path %s: %w", path, err)
		}
		return candidate, nil
	}
	return "", fmt.Errorf("could not find available suffix for %q (tried 1-%d); use --as to specify a name", task, maxDuplicateSuffix)
}
