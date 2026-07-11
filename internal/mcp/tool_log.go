package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// mcpLogEntry mirrors cmd/log.go's logEntry — MCP can't import package cmd.
type mcpLogEntry struct {
	branch     string
	task       string
	service    string
	typeName   string
	path       string
	commitTime time.Time
	subject    string
	valid      bool
}

func registerLogTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("log",
		mcp.WithDescription("Show the last commit from each worktree, sorted by recency"),
		mcp.WithInteger("limit",
			mcp.Description("Maximum number of entries to return (0 = all)"),
		),
		mcp.WithString("since",
			mcp.Description("Only show entries since this duration ago, e.g. '7d', '2w', '3h'"),
		),
	)
	s.AddTool(tool, handleLog(hctx))
}

// handleLog intentionally skips requireConfig(): mirrors the CLI's skipConfig
// annotation so log still works in an uninitialized repo.
func handleLog(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r := hctx.Runner

		mainBranch, err := git.DefaultBranch(ctx, r)
		if err != nil {
			return errorResult(err), nil
		}

		entries, err := git.ListWorktrees(ctx, r)
		if err != nil {
			return errorResult(err), nil
		}

		candidates := git.FilterEntries(entries, mainBranch)
		if len(candidates) == 0 {
			return marshalResult(logResult{Entries: []logItem{}})
		}

		valid := collectMCPLogEntries(ctx, r, candidates, hctx.PrefixSet().Strip())

		valid, err = filterLogEntriesSince(valid, req.GetString("since", ""))
		if err != nil {
			return errorResult(err), nil
		}

		if limit := req.GetInt("limit", 0); limit > 0 && len(valid) > limit {
			valid = valid[:limit]
		}

		return marshalResult(logResult{Entries: toLogItems(valid)})
	}
}

// filterLogEntriesSince drops entries older than sinceStr (e.g. "7d"); empty is a no-op.
func filterLogEntriesSince(valid []mcpLogEntry, sinceStr string) ([]mcpLogEntry, error) {
	if sinceStr == "" {
		return valid, nil
	}
	d, err := resolver.ParseDuration(sinceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid since value %q: %w", sinceStr, err)
	}
	cutoff := time.Now().Add(-d)
	filtered := make([]mcpLogEntry, 0, len(valid))
	for _, e := range valid {
		if e.commitTime.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// collectMCPLogEntries gathers commit info per candidate in parallel, sorted by recency.
func collectMCPLogEntries(ctx context.Context, r git.Runner, candidates []git.WorktreeEntry, prefixes []string) []mcpLogEntry {
	results := parallel.Collect(ctx, len(candidates), 8, func(ctx context.Context, i int) mcpLogEntry {
		itemCtx, cancel := git.WithItemTimeout(ctx)
		defer cancel()
		e := candidates[i]
		svc, task, matchedPrefix := resolver.ServiceFromBranch(e.Branch, prefixes)
		typeName := strings.TrimSuffix(matchedPrefix, "/")

		ct, subject, err := git.LastCommitInfo(itemCtx, r, e.Branch)
		if err != nil {
			return mcpLogEntry{branch: e.Branch, task: task, service: svc, typeName: typeName, path: e.Path}
		}
		return mcpLogEntry{
			branch:     e.Branch,
			task:       task,
			service:    svc,
			typeName:   typeName,
			path:       e.Path,
			commitTime: ct,
			subject:    subject,
			valid:      true,
		}
	})

	valid := make([]mcpLogEntry, 0, len(results))
	for _, res := range results {
		if res.valid {
			valid = append(valid, res)
		}
	}

	sort.Slice(valid, func(i, j int) bool {
		return valid[i].commitTime.After(valid[j].commitTime)
	})

	return valid
}

func toLogItems(entries []mcpLogEntry) []logItem {
	items := make([]logItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, logItem{
			Task:       e.task,
			Service:    e.service,
			Type:       e.typeName,
			Branch:     e.branch,
			Path:       e.path,
			LastCommit: e.commitTime.UTC().Format(time.RFC3339),
			Subject:    e.subject,
		})
	}
	return items
}
