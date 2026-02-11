package conflict

// MergePlan represents the suggested merge order.
type MergePlan struct {
	Steps []MergeStep
}

// MergeStep is a single branch in the recommended merge order.
type MergeStep struct {
	Branch       string
	OverlapCount int // number of file overlaps with other unmerged branches
}

// Plan computes a greedy merge order that minimizes conflict risk.
// Branches with fewer overlaps are merged first.
func Plan(analysis *Analysis) *MergePlan {
	if analysis == nil || len(analysis.Pairs) == 0 {
		return &MergePlan{}
	}

	remaining := collectBranches(analysis)
	var steps []MergeStep

	for len(remaining) > 0 {
		counts := countOverlaps(analysis.Pairs, remaining)
		best, bestCount := pickLowest(remaining, counts)
		steps = append(steps, MergeStep{Branch: best, OverlapCount: bestCount})
		delete(remaining, best)
	}

	return &MergePlan{Steps: steps}
}

// collectBranches returns all branches involved in any overlap.
func collectBranches(analysis *Analysis) map[string]struct{} {
	branches := make(map[string]struct{})
	for _, p := range analysis.Pairs {
		branches[p.BranchA] = struct{}{}
		branches[p.BranchB] = struct{}{}
	}
	return branches
}

// countOverlaps tallies file overlaps for each remaining branch.
func countOverlaps(pairs []PairConflict, remaining map[string]struct{}) map[string]int {
	counts := make(map[string]int)
	for _, p := range pairs {
		_, aIn := remaining[p.BranchA]
		_, bIn := remaining[p.BranchB]
		if aIn && bIn {
			counts[p.BranchA] += len(p.OverlapFiles)
			counts[p.BranchB] += len(p.OverlapFiles)
		}
	}
	return counts
}

// pickLowest selects the branch with the fewest overlaps, breaking ties alphabetically.
func pickLowest(remaining map[string]struct{}, counts map[string]int) (string, int) {
	best := ""
	bestCount := -1
	for b := range remaining {
		c := counts[b]
		if best == "" || c < bestCount || (c == bestCount && b < best) {
			best = b
			bestCount = c
		}
	}
	return best, bestCount
}
