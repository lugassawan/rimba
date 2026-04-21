package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// safeNameRe matches strings safe to embed in git remote names and URLs.
var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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
		return PRMeta{}, fmt.Errorf("fetch PR #%d: %w", num, err)
	}
	var resp prViewResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return PRMeta{}, fmt.Errorf("parse PR #%d metadata: %w", num, err)
	}
	if resp.HeadRefName == "" {
		return PRMeta{}, fmt.Errorf("PR #%d: missing headRefName in response", num)
	}
	owner := resp.HeadRepositoryOwner.Login
	repoName := resp.HeadRepository.Name
	if owner == "" {
		return PRMeta{}, fmt.Errorf("PR #%d: missing headRepositoryOwner in response", num)
	}
	if repoName == "" {
		return PRMeta{}, fmt.Errorf("PR #%d: missing headRepository in response", num)
	}
	if !safeNameRe.MatchString(owner) {
		return PRMeta{}, fmt.Errorf("PR #%d: unsafe headRepositoryOwner %q", num, owner)
	}
	if !safeNameRe.MatchString(repoName) {
		return PRMeta{}, fmt.Errorf("PR #%d: unsafe headRepository.name %q", num, repoName)
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
