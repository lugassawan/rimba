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

	// GhUnavailableWarning is surfaced when --full is requested but gh is
	// missing or unauthenticated. Callers render it verbatim.
	GhUnavailableWarning = "gh unavailable; PR/CI columns blank"
)

// ListWorktreesRequest configures ListWorktrees.
type ListWorktreesRequest struct {
	// Full enables PR/CI lookup. Requires a non-nil ghR; when Full is true
	// and ghR is nil, the use case behaves as if Full were false.
	Full bool
	// TypeFilter keeps only worktrees whose branch prefix matches (e.g. "feature").
	// Empty string means no filter. Caller validates before passing.
	TypeFilter string
	// Dirty keeps only worktrees with uncommitted changes.
	Dirty bool
	// Behind keeps only worktrees behind upstream.
	Behind bool
	// Service keeps only worktrees with matching service (monorepo).
	Service string
	// CurrentPath marks the worktree whose resolved path equals this value as
	// IsCurrent. Empty string means no worktree is marked.
	CurrentPath string
	// WorktreeDir is the absolute directory holding worktrees; used to
	// compute a relative display path when shorter than the absolute.
	WorktreeDir string
}

// PRInfo is the per-branch PR/CI summary.
//
// Three states matter to consumers:
//   - PRInfos map is nil         → gh not queried (Full off or auth failed)
//   - PRInfos[branch].Number == 0 → queried, no open PR for this branch
//   - PRInfos[branch].Number  > 0 → known PR; CIStatus may still be "" when
//     the PR has no checks reported.
type PRInfo struct {
	Number   int
	CIStatus gh.CIStatus
}

// ListWorktreesResult carries the shaped rows plus optional PR data and
// a user-facing gh warning.
type ListWorktreesResult struct {
	Rows      []resolver.WorktreeDetail
	PRInfos   map[string]PRInfo
	GhWarning string
}

// ListWorktrees is the shared pipeline used by `rimba list` and the MCP list
// tool: candidate filter → optional gh auth check → parallel status (+PR)
// collection → dirty/behind filter → service filter → sort.
//
// ghR is nil when the caller cannot or does not want to query gh. When
// req.Full is true and ghR is non-nil but auth fails, the use case falls
// back to the non-full path and populates GhWarning.
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

	// activeGhR is captured by value into each worker closure below —
	// reassignment above happens-before goroutine start, but an explicit
	// local makes the intent obvious and keeps the race detector happy
	// if the sequence is ever reordered.
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

// listCandidate is a pre-filtered worktree entry paired with its display
// path and current-worktree marker.
type listCandidate struct {
	entry       git.WorktreeEntry
	displayPath string
	isCurrent   bool
}

// listWorktreeResult pairs a resolved detail with its optional PR info
// during the parallel collect stage.
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
