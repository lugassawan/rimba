package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/executor"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerExecTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("exec",
		mcp.WithDescription("Run a shell command across matching worktrees in parallel"),
		mcp.WithString("command",
			mcp.Description("Shell command to run in each worktree"),
			mcp.Required(),
		),
		mcp.WithBoolean("all",
			mcp.Description("Target all eligible worktrees"),
		),
		mcp.WithString("type",
			mcp.Description("Filter by prefix type (feature, bugfix, hotfix, docs, test, chore)"),
			mcp.Enum("feature", "bugfix", "hotfix", "docs", "test", "chore"),
		),
		mcp.WithBoolean("dirty",
			mcp.Description("Only run in worktrees with uncommitted changes"),
		),
		mcp.WithBoolean("fail_fast",
			mcp.Description("Stop on first failure"),
		),
		mcp.WithNumber("concurrency",
			mcp.Description("Max parallel executions (0 = unlimited)"),
		),
	)
	s.AddTool(tool, handleExec(hctx))
}

func handleExec(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		command := req.GetString("command", "")
		if command == "" {
			return mcp.NewToolResultError("command is required"), nil
		}

		all := req.GetBool("all", false)
		typeFilter := req.GetString("type", "")
		dirty := req.GetBool("dirty", false)
		failFast := req.GetBool("fail_fast", false)
		concurrency := req.GetInt("concurrency", 0)

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return mcp.NewToolResultError(cfgErr.Error()), nil
		}

		if !all && typeFilter == "" {
			return mcp.NewToolResultError("provide all=true or type to select worktrees"), nil
		}

		if typeFilter != "" && !resolver.ValidPrefixType(typeFilter) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid type %q; valid types: feature, bugfix, hotfix, docs, test, chore", typeFilter)), nil
		}

		filtered, err := resolveExecTargets(hctx.Runner, cfg, typeFilter, dirty)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(filtered) == 0 {
			return marshalResult(execData{
				Command: command,
				Results: make([]execResult, 0),
				Success: true,
			})
		}

		return runExecCommand(ctx, command, filtered, concurrency, failFast)
	}
}

// resolveExecTargets collects and filters worktrees for exec.
func resolveExecTargets(r git.Runner, cfg *config.Config, typeFilter string, dirty bool) ([]resolver.WorktreeInfo, error) {
	worktrees, err := operations.ListWorktreeInfos(r)
	if err != nil {
		return nil, err
	}

	prefixes := resolver.AllPrefixes()

	var filtered []resolver.WorktreeInfo
	if typeFilter != "" {
		filtered = operations.FilterByType(worktrees, prefixes, typeFilter)
	} else {
		allTasks := operations.CollectTasks(worktrees, prefixes)
		filtered = operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)
	}

	if dirty {
		filtered = filterDirty(r, filtered)
	}

	return filtered, nil
}

// runExecCommand executes a command across worktrees and returns the result.
func runExecCommand(ctx context.Context, command string, filtered []resolver.WorktreeInfo, concurrency int, failFast bool) (*mcp.CallToolResult, error) {
	prefixes := resolver.AllPrefixes()

	targets := make([]executor.Target, len(filtered))
	for i, wt := range filtered {
		task, _ := resolver.TaskFromBranch(wt.Branch, prefixes)
		targets[i] = executor.Target{
			Path:   wt.Path,
			Branch: wt.Branch,
			Task:   task,
		}
	}

	results := executor.Run(ctx, executor.Config{
		Targets:     targets,
		Command:     command,
		Concurrency: concurrency,
		FailFast:    failFast,
		Runner:      executor.ShellRunner(),
	})

	return marshalResult(buildExecData(command, results))
}

// buildExecData converts executor results into the JSON response.
func buildExecData(command string, results []executor.Result) execData {
	jsonResults := make([]execResult, len(results))
	allOK := true
	for i, res := range results {
		jr := execResult{
			Task:      res.Target.Task,
			Branch:    res.Target.Branch,
			Path:      res.Target.Path,
			ExitCode:  res.ExitCode,
			Stdout:    string(res.Stdout),
			Stderr:    string(res.Stderr),
			Cancelled: res.Cancelled,
		}
		if res.Err != nil {
			jr.Error = res.Err.Error()
		}
		if res.ExitCode != 0 || res.Err != nil {
			allOK = false
		}
		jsonResults[i] = jr
	}
	return execData{
		Command: command,
		Results: jsonResults,
		Success: allOK,
	}
}

// filterDirty filters worktrees to only those with uncommitted changes.
func filterDirty(r git.Runner, worktrees []resolver.WorktreeInfo) []resolver.WorktreeInfo {
	isDirtyFlags := make([]bool, len(worktrees))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i, wt := range worktrees {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			d, err := git.IsDirty(r, path)
			if err == nil && d {
				isDirtyFlags[idx] = true
			}
		}(i, wt.Path)
	}
	wg.Wait()

	var out []resolver.WorktreeInfo
	for i, wt := range worktrees {
		if isDirtyFlags[i] {
			out = append(out, wt)
		}
	}
	return out
}
