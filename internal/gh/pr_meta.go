package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

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
	return PRMeta{
		Number:            resp.Number,
		Title:             resp.Title,
		HeadRefName:       resp.HeadRefName,
		HeadRepoOwner:     resp.HeadRepositoryOwner.Login,
		HeadRepoName:      resp.HeadRepository.Name,
		IsCrossRepository: resp.IsCrossRepository,
	}, nil
}
