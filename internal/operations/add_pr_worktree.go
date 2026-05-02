package operations

import (
	"context"
	"fmt"
	"strconv"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
	"github.com/lugassawan/rimba/internal/resolver"
)

// AddPRParams holds the inputs for creating a worktree from a GitHub PR.
type AddPRParams struct {
	PRNumber     int
	TaskOverride string // empty = derive from PR title
	PostCreateOptions
}

// AddPRWorktree creates a worktree from a GitHub PR's head branch.
// For cross-fork PRs it adds a fork remote (gh-fork-<owner>) if absent,
// then delegates to AddWorktree for copy-files, deps, and hooks.
func AddPRWorktree(
	ctx context.Context,
	gitR git.Runner,
	ghR gh.Runner,
	params AddPRParams,
	onProgress progress.Func,
) (AddResult, error) {
	if err := gh.CheckAuth(ctx, ghR); err != nil {
		return AddResult{}, err
	}

	progress.Notify(onProgress, fmt.Sprintf("Fetching PR #%d metadata...", params.PRNumber))
	meta, err := gh.FetchPRMeta(ctx, ghR, params.PRNumber)
	if err != nil {
		return AddResult{}, err
	}

	task := params.TaskOverride
	if task == "" {
		task = "review/" + strconv.Itoa(meta.Number) + "-" + resolver.Slugify(meta.Title)
	}

	source, err := resolveSource(gitR, meta, onProgress)
	if err != nil {
		return AddResult{}, err
	}

	return AddWorktree(gitR, AddParams{
		Task:              task,
		Source:            source,
		PostCreateOptions: params.PostCreateOptions,
	}, onProgress)
}

// resolveSource fetches the appropriate remote ref and returns the source ref
// for use with git worktree add -b <branch> <path> <source>.
func resolveSource(gitR git.Runner, meta gh.PRMeta, onProgress progress.Func) (string, error) {
	if !meta.IsCrossRepository {
		progress.Notify(onProgress, "Fetching origin...")
		if err := git.Fetch(gitR, "origin"); err != nil {
			return "", errhint.WithFix(err, "check network connectivity")
		}
		return "origin/" + meta.HeadRefName, nil
	}

	remoteName := "gh-fork-" + meta.HeadRepoOwner
	remoteURL := "https://github.com/" + meta.HeadRepoOwner + "/" + meta.HeadRepoName + ".git"

	if !git.RemoteExists(gitR, remoteName) {
		progress.Notify(onProgress, fmt.Sprintf("Adding fork remote %s...", remoteName))
		if err := git.AddRemote(gitR, remoteName, remoteURL); err != nil {
			return "", errhint.WithFix(err, "check network and fork visibility")
		}
	}

	progress.Notify(onProgress, fmt.Sprintf("Fetching %s...", remoteName))
	if err := git.Fetch(gitR, remoteName); err != nil {
		return "", errhint.WithFix(err, "check network and fork visibility")
	}

	return remoteName + "/" + meta.HeadRefName, nil
}
