package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
)

// safeNameRe matches strings safe to embed in git remote names and URLs.
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

const ghShapeHint = "gh CLI returned an unexpected response shape — update gh: gh --version"

// PRMeta holds the metadata needed to check out a PR's head branch.
type PRMeta struct {
	Number            int
	Title             string
	HeadRefName       string
	HeadRepoOwner     string
	HeadRepoName      string
	IsCrossRepository bool
}

type prViewResponse struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	HeadRefName    string `json:"headRefName"`
	HeadRepository struct {
		Name string `json:"name"`
	} `json:"headRepository"`
	HeadRepositoryOwner struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
	IsCrossRepository bool `json:"isCrossRepository"`
}

// FetchPRMeta retrieves metadata for the given PR number using the gh CLI.
func FetchPRMeta(ctx context.Context, r Runner, num int) (PRMeta, error) {
	out, err := r.Run(ctx, "pr", "view", strconv.Itoa(num),
		"--json", "number,title,headRefName,headRepository,headRepositoryOwner,isCrossRepository",
	)
	if err != nil {
		fetchErr := fmt.Errorf("fetch PR #%d: %w", num, err)
		return PRMeta{}, errhint.WithFix(fetchErr, classifyFetchPRErr(err))
	}
	var resp prViewResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return PRMeta{}, errhint.WithFix(fmt.Errorf("parse PR #%d metadata: %w", num, err), ghShapeHint)
	}
	if resp.HeadRefName == "" {
		return PRMeta{}, errhint.WithFix(fmt.Errorf("PR #%d: missing headRefName in response", num), ghShapeHint)
	}
	owner := resp.HeadRepositoryOwner.Login
	repoName := resp.HeadRepository.Name
	if owner == "" {
		return PRMeta{}, errhint.WithFix(fmt.Errorf("PR #%d: missing headRepositoryOwner in response", num), ghShapeHint)
	}
	if repoName == "" {
		return PRMeta{}, errhint.WithFix(fmt.Errorf("PR #%d: missing headRepository in response", num), ghShapeHint)
	}
	if !safeNameRe.MatchString(owner) {
		return PRMeta{}, errhint.WithFix(fmt.Errorf("PR #%d: unsafe headRepositoryOwner %q", num, owner), ghShapeHint)
	}
	if !safeNameRe.MatchString(repoName) {
		return PRMeta{}, errhint.WithFix(fmt.Errorf("PR #%d: unsafe headRepository.name %q", num, repoName), ghShapeHint)
	}
	return PRMeta{
		Number:            resp.Number,
		Title:             resp.Title,
		HeadRefName:       resp.HeadRefName,
		HeadRepoOwner:     owner,
		HeadRepoName:      repoName,
		IsCrossRepository: resp.IsCrossRepository,
	}, nil
}

func classifyFetchPRErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "404") || strings.Contains(msg, "Could not resolve"):
		return "verify PR number and repo access: gh pr view <num>"
	case strings.Contains(msg, "rate limit"):
		return "GitHub API rate limit hit — wait or set GITHUB_TOKEN"
	default:
		return "check network access and gh auth: gh auth status"
	}
}
