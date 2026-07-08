package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

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
			mcp.Description("Filter by prefix type (built-in: feature, bugfix, hotfix, docs, test, chore; or any custom type configured in [[resolver.prefix]])"),
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

func handleList(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		typeFilter := req.GetString("type", "")
		dirty := req.GetBool("dirty", false)
		behind := req.GetBool("behind", false)
		archived := req.GetBool("archived", false)

		r := hctx.Runner

		if archived {
			return handleListArchived(ctx, r, hctx)
		}

		cfg, err := hctx.requireConfig()
		if err != nil {
			return errorResult(err), nil
		}

		ps := cfg.PrefixSet()
		if typeFilter != "" && !ps.ValidType(typeFilter) {
			return invalidTypeResult(typeFilter, ps, ""), nil
		}

		res, err := operations.ListWorktrees(ctx, r, nil, operations.ListWorktreesRequest{
			TypeFilter:  typeFilter,
			Dirty:       dirty,
			Behind:      behind,
			WorktreeDir: filepath.Join(hctx.RepoRoot, cfg.WorktreeDir),
		})
		if err != nil {
			return errorResult(err), nil
		}

		return marshalResult(detailsToListItems(res.Rows))
	}
}

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

func handleListArchived(ctx context.Context, r git.Runner, hctx *HandlerContext) (*mcp.CallToolResult, error) {
	mainBranch, err := operations.ResolveMainBranch(ctx, r, configDefault(hctx))
	if err != nil {
		return errorResult(err), nil
	}

	archived, err := operations.ListArchivedBranches(ctx, r, mainBranch)
	if err != nil {
		return errorResult(err), nil
	}

	prefixes := hctx.PrefixSet().Strip()
	items := make([]listArchivedItem, 0, len(archived))
	for _, b := range archived {
		task, typeName := resolver.TaskAndType(b, prefixes)
		items = append(items, listArchivedItem{Task: task, Type: typeName, Branch: b})
	}

	return marshalResult(items)
}

func configDefault(hctx *HandlerContext) string {
	if hctx.Config != nil {
		return hctx.Config.DefaultSource
	}
	return ""
}

func marshalResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return errorResult(fmt.Errorf("failed to marshal result: %w", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
