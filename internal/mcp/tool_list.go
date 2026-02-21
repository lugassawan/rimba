package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerListTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("list",
		mcp.WithDescription("List all worktrees with their branch, path, and status"),
		mcp.WithString("type",
			mcp.Description("Filter by prefix type (feature, bugfix, hotfix, docs, test, chore)"),
			mcp.Enum("feature", "bugfix", "hotfix", "docs", "test", "chore"),
		),
		mcp.WithBoolean("dirty",
			mcp.Description("Only show worktrees with uncommitted changes"),
		),
		mcp.WithBoolean("behind",
			mcp.Description("Only show worktrees behind upstream"),
		),
		mcp.WithBoolean("archived",
			mcp.Description("Show archived branches instead of active worktrees"),
		),
	)
	s.AddTool(tool, handleList(hctx))
}

// listCandidate holds a worktree entry with its display path for list filtering.
type listCandidate struct {
	entry       git.WorktreeEntry
	displayPath string
}

func handleList(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		typeFilter := req.GetString("type", "")
		dirty := req.GetBool("dirty", false)
		behind := req.GetBool("behind", false)
		archived := req.GetBool("archived", false)

		r := hctx.Runner

		if archived {
			return handleListArchived(r, hctx)
		}

		cfg, err := hctx.requireConfig()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if typeFilter != "" && !resolver.ValidPrefixType(typeFilter) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid type %q; valid types: feature, bugfix, hotfix, docs, test, chore", typeFilter)), nil
		}

		entries, err := git.ListWorktrees(r)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(entries) == 0 {
			return marshalResult(make([]listItem, 0))
		}

		wtDir := filepath.Join(hctx.RepoRoot, cfg.WorktreeDir)
		prefixes := resolver.AllPrefixes()

		candidates := filterListCandidates(entries, wtDir, typeFilter, prefixes)

		rows := make([]resolver.WorktreeDetail, len(candidates))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)

		for i, c := range candidates {
			wg.Add(1)
			go func(idx int, c listCandidate) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				status := operations.CollectWorktreeStatus(r, c.entry.Path)
				rows[idx] = resolver.NewWorktreeDetail(c.entry.Branch, prefixes, c.displayPath, status, false)
			}(i, c)
		}
		wg.Wait()

		rows = operations.FilterDetailsByStatus(rows, dirty, behind)
		resolver.SortDetailsByTask(rows)

		return marshalResult(detailsToListItems(rows))
	}
}

// detailsToListItems converts worktree details to list items.
func detailsToListItems(rows []resolver.WorktreeDetail) []listItem {
	items := make([]listItem, len(rows))
	for i, row := range rows {
		items[i] = listItem{
			Task:      row.Task,
			Type:      row.Type,
			Branch:    row.Branch,
			Path:      row.Path,
			IsCurrent: false,
			Status:    row.Status,
		}
	}
	return items
}

// filterListCandidates filters worktree entries by type and computes display paths.
func filterListCandidates(entries []git.WorktreeEntry, wtDir, typeFilter string, prefixes []string) []listCandidate {
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

		candidates = append(candidates, listCandidate{entry: e, displayPath: displayPath})
	}
	return candidates
}

func handleListArchived(r git.Runner, hctx *HandlerContext) (*mcp.CallToolResult, error) {
	mainBranch, err := operations.ResolveMainBranch(r, configDefault(hctx))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	archived, err := operations.ListArchivedBranches(r, mainBranch)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	prefixes := resolver.AllPrefixes()
	items := make([]listArchivedItem, 0, len(archived))
	for _, b := range archived {
		task, matchedPrefix := resolver.TaskFromBranch(b, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")
		items = append(items, listArchivedItem{Task: task, Type: typeName, Branch: b})
	}

	return marshalResult(items)
}

// configDefault returns the default_source from config, or "" if unset.
func configDefault(hctx *HandlerContext) string {
	if hctx.Config != nil {
		return hctx.Config.DefaultSource
	}
	return ""
}

// marshalResult serializes data to JSON and wraps it in a tool result.
func marshalResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
