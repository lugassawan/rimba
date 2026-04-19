package operations

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
)

const (
	listWorktreesConcurrency = 8
	prQueryTimeout           = 10 * time.Second

	GhUnavailableWarning = "gh unavailable; PR/CI columns blank"
)

// ListWorktreesRequest configures ListWorktrees.
type ListWorktreesRequest struct {
	Full        bool   // include PR/CI (requires non-nil ghR)
	TypeFilter  string // branch prefix, e.g. "feature"
	Dirty       bool
	Behind      bool
	Service     string // monorepo service filter
	CurrentPath string // marks matching worktree as IsCurrent; "" to skip
	WorktreeDir string // absolute; used to shorten display paths
}

// PRInfo is the per-branch PR/CI summary.
//
// Encoding: the PRInfos map is nil when gh was not queried. Within the map,
// Number == 0 means no open PR for that branch; Number > 0 means a known PR
// where CIStatus may still be empty (no checks reported).
type PRInfo struct {
	Number   int
	CIStatus gh.CIStatus
}

// ListWorktreesResult carries the shaped rows with optional PR data and a gh warning.
type ListWorktreesResult struct {
	Rows      []resolver.WorktreeDetail
	PRInfos   map[string]PRInfo
	GhWarning string
}

// ListWorktrees is the shared pipeline for `rimba list` and the MCP list tool:
// candidate filter → optional gh auth check → parallel status (+PR) collection
// → dirty/behind filter → service filter → sort.
//
// Pass ghR=nil to skip PR/CI lookup. When req.Full is true and auth fails, the
// use case falls back silently and populates GhWarning.
func ListWorktrees(
	ctx context.Context,
	gitR git.Runner,
	ghR gh.Runner,
	req ListWorktreesRequest,
) (ListWorktreesResult, error) {
	entries, err := git.ListWorktrees(gitR)
	if err != nil {
		return ListWorktreesResult{}, err
	}

	prefixes := resolver.AllPrefixes()
	candidates := buildListCandidates(entries, req.WorktreeDir, req.CurrentPath, req.TypeFilter, prefixes)

	var (
		prInfos   map[string]PRInfo
		ghWarning string
		activeGhR = ghR
	)
	if req.Full && activeGhR != nil {
		if err := gh.CheckAuth(ctx, activeGhR); err != nil {
			ghWarning = GhUnavailableWarning
			activeGhR = nil
		} else {
			prInfos = make(map[string]PRInfo, len(candidates))
		}
	}

	results := parallel.Collect(len(candidates), listWorktreesConcurrency, func(i int) listWorktreeResult {
		c := candidates[i]
		status := CollectWorktreeStatus(gitR, c.entry.Path)
		d := resolver.NewWorktreeDetail(c.entry.Branch, prefixes, c.displayPath, status, c.isCurrent)
		var info PRInfo
		if activeGhR != nil {
			info = queryPRInfo(ctx, activeGhR, c.entry.Branch)
		}
		return listWorktreeResult{detail: d, info: info}
	})

	rows := make([]resolver.WorktreeDetail, len(results))
	for i, res := range results {
		rows[i] = res.detail
		if prInfos != nil {
			prInfos[res.detail.Branch] = res.info
		}
	}

	rows = FilterDetailsByStatus(rows, req.Dirty, req.Behind)
	rows = resolver.FilterByService(rows, req.Service)
	resolver.SortDetailsByTask(rows)

	return ListWorktreesResult{Rows: rows, PRInfos: prInfos, GhWarning: ghWarning}, nil
}

type listCandidate struct {
	entry       git.WorktreeEntry
	displayPath string
	isCurrent   bool
}

type listWorktreeResult struct {
	detail resolver.WorktreeDetail
	info   PRInfo
}

func buildListCandidates(entries []git.WorktreeEntry, wtDir, currentPath, typeFilter string, prefixes []string) []listCandidate {
	currentResolved := ""
	if currentPath != "" {
		r, _ := filepath.EvalSymlinks(currentPath)
		currentResolved = filepath.Clean(r)
	}

	var candidates []listCandidate
	for _, e := range entries {
		if e.Bare {
			continue
		}

		if typeFilter != "" {
			_, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)
			entryType := strings.TrimSuffix(matchedPrefix, "/")
			if entryType != typeFilter {
				continue
			}
		}

		displayPath := e.Path
		if rel, err := filepath.Rel(wtDir, e.Path); err == nil && len(rel) < len(displayPath) {
			displayPath = rel
		}

		isCurrent := false
		if currentResolved != "" {
			entryResolved, _ := filepath.EvalSymlinks(e.Path)
			isCurrent = currentResolved == filepath.Clean(entryResolved)
		}

		candidates = append(candidates, listCandidate{entry: e, displayPath: displayPath, isCurrent: isCurrent})
	}
	return candidates
}

// queryPRInfo runs one gh pr list under a timeout. Errors degrade silently
// so one slow or broken query does not fail the whole table.
func queryPRInfo(ctx context.Context, ghR gh.Runner, branch string) PRInfo {
	qctx, cancel := context.WithTimeout(ctx, prQueryTimeout)
	defer cancel()
	pr, err := gh.QueryPRStatus(qctx, ghR, branch)
	if err != nil || pr.Number == 0 {
		return PRInfo{}
	}
	return PRInfo{Number: pr.Number, CIStatus: pr.CIStatus}
}
