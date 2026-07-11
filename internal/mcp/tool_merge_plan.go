package mcp

import (
	"context"

	"github.com/lugassawan/rimba/internal/conflict"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMergePlanTool(s *server.MCPServer, hctx *HandlerContext) {
	tool := mcp.NewTool("merge-plan",
		mcp.WithDescription("Recommend a merge order for active worktree branches that minimizes conflicts"),
	)
	s.AddTool(tool, handleMergePlan(hctx))
}

func handleMergePlan(hctx *HandlerContext) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg, err := hctx.requireConfig()
		if err != nil {
			return errorResult(err), nil
		}

		r := hctx.Runner

		worktrees, err := operations.ListWorktreeInfos(ctx, r)
		if err != nil {
			return errorResult(err), nil
		}

		prefixes := cfg.PrefixSet().Strip()
		allTasks := operations.CollectTasks(worktrees, prefixes)
		eligible := operations.FilterEligible(worktrees, prefixes, cfg.DefaultSource, allTasks, true)

		if len(eligible) == 0 {
			return marshalResult(mergePlanResult{Steps: []mergePlanStep{}})
		}

		diffs, err := conflict.CollectDiffs(ctx, r, cfg.DefaultSource, eligible)
		if err != nil {
			return errorResult(err), nil
		}

		overlapResult := conflict.DetectOverlaps(diffs)

		branchNames := make([]string, len(eligible))
		for i, wt := range eligible {
			branchNames[i] = wt.Branch
		}

		steps := conflict.PlanMergeOrder(overlapResult.Overlaps, branchNames)

		return marshalResult(mergePlanResult{Steps: toMergePlanSteps(steps, prefixes)})
	}
}

// toMergePlanSteps converts conflict.MergeStep values to JSON-ready mergePlanStep values.
func toMergePlanSteps(steps []conflict.MergeStep, prefixes []string) []mergePlanStep {
	result := make([]mergePlanStep, len(steps))
	for i, step := range steps {
		task, _ := resolver.TaskAndType(step.Branch, prefixes)
		result[i] = mergePlanStep{
			Order:     step.Order,
			Task:      task,
			Branch:    step.Branch,
			Conflicts: step.Conflicts,
		}
	}
	return result
}
