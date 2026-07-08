package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/executor"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/parallel"
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
			mcp.Description("Filter by prefix type (built-in: feature, bugfix, hotfix, docs, test, chore; or any custom type configured in [[resolver.prefix]])"),
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

// handleExec runs ad-hoc caller-supplied shell commands across worktrees.
// The trust consent gate (GateNonInteractive) is intentionally absent here:
// `exec` executes commands provided at call time by the MCP client, not
// committed configuration hooks. Gating committed hooks is the threat model
// for the trust system; ad-hoc operator commands are considered authorised by
// the act of invoking the MCP server.
func handleExec(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		command := req.GetString("command", "")
		if command == "" {
			return errorResult(errhint.WithFix(errors.New("command is required"),
				`provide the command argument, e.g. exec { command: "npm test", all: true }`)), nil
		}

		all := req.GetBool("all", false)
		typeFilter := req.GetString("type", "")
		dirty := req.GetBool("dirty", false)
		failFast := req.GetBool("fail_fast", false)
		concurrency := req.GetInt("concurrency", 0)

		cfg, cfgErr := hctx.requireConfig()
		if cfgErr != nil {
			return errorResult(cfgErr), nil
		}

		if !all && typeFilter == "" {
			return errorResult(errhint.WithFix(errors.New("provide all=true or type to select worktrees"),
				"set all=true to target every worktree, or pass type=<prefix>")), nil
		}

		ps := cfg.PrefixSet()
		if typeFilter != "" && !ps.ValidType(typeFilter) {
			return errorResult(errhint.WithFix(
				fmt.Errorf("invalid type %q", typeFilter),
				"use one of: "+strings.Join(ps.TypeNames(), ", "),
			)), nil
		}

		filtered, err := resolveExecTargets(ctx, hctx.Runner, cfg, typeFilter, dirty)
		if err != nil {
			return errorResult(err), nil
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
func resolveExecTargets(ctx context.Context, r git.Runner, cfg *config.Config, typeFilter string, dirty bool) ([]resolver.WorktreeInfo, error) {
	worktrees, err := operations.ListWorktreeInfos(ctx, r)
	if err != nil {
		return nil, err
	}

	ps := cfg.PrefixSet()

	var filtered []resolver.WorktreeInfo
	if typeFilter != "" {
		filtered = operations.FilterByType(worktrees, ps, typeFilter)
	} else {
		prefixes := ps.Strip()
		allTasks := operations.CollectTasks(worktrees, prefixes)
		filtered = operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)
	}

	if dirty {
		filtered = filterDirty(ctx, r, filtered)
	}

	filtered = excludeOrphanedExec(filtered, ps, cfg.DefaultSource)

	return filtered, nil
}

// excludeOrphanedExec drops worktrees whose branch was created under a custom
// prefix that is no longer configured, warning to os.Stderr (mirroring
// filterDirty's warning channel) when any are excluded.
func excludeOrphanedExec(worktrees []resolver.WorktreeInfo, ps *resolver.PrefixSet, mainBranch string) []resolver.WorktreeInfo {
	if !ps.HasCustom() {
		return worktrees
	}

	var kept []resolver.WorktreeInfo
	var excluded int
	for _, wt := range worktrees {
		if ps.IsOrphan(wt.Branch, mainBranch) {
			excluded++
			continue
		}
		kept = append(kept, wt)
	}

	if excluded > 0 {
		fmt.Fprintf(os.Stderr,
			"Warning: excluding %d worktree(s) with an unrecognized prefix (re-add it to [[resolver.prefix]] to include them)\n",
			excluded)
	}

	return kept
}

// runExecCommand executes a command across worktrees and returns the result.
func runExecCommand(ctx context.Context, command string, filtered []resolver.WorktreeInfo, concurrency int, failFast bool) (*mcp.CallToolResult, error) {
	prefixes := config.PrefixSetFromContext(ctx).Strip()

	targets := make([]executor.Target, len(filtered))
	for i, wt := range filtered {
		task, _ := resolver.PureTaskFromBranch(wt.Branch, prefixes)
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

type dirtyCheckResult struct {
	dirty   bool
	warning string
}

// filterDirty filters worktrees to only those with uncommitted changes.
// If IsDirty returns an error, the worktree is treated as dirty (included)
// and a warning is written to os.Stderr so the operator can investigate.
func filterDirty(ctx context.Context, r git.Runner, worktrees []resolver.WorktreeInfo) []resolver.WorktreeInfo {
	results := parallel.Collect(ctx, len(worktrees), 8, func(ctx context.Context, i int) dirtyCheckResult {
		itemCtx, cancel := git.WithItemTimeout(ctx)
		defer cancel()
		d, err := git.IsDirty(itemCtx, r, worktrees[i].Path)
		if err != nil {
			return dirtyCheckResult{dirty: true, warning: fmt.Sprintf("Warning: cannot check dirty status for %s: %v", worktrees[i].Path, err)}
		}
		return dirtyCheckResult{dirty: d}
	})

	for _, res := range results {
		if res.warning != "" {
			fmt.Fprintln(os.Stderr, res.warning)
		}
	}

	var out []resolver.WorktreeInfo
	for i, res := range results {
		if res.dirty {
			out = append(out, worktrees[i])
		}
	}
	return out
}
