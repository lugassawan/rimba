package conflict

import "slices"

// MergeStep represents one step in the recommended merge order.
type MergeStep struct {
	Order     int
	Branch    string
	Conflicts int
}

// PlanMergeOrder computes a greedy merge order that minimizes conflicts at each step.
// It builds an NxN conflict matrix from overlaps, then repeatedly picks the branch
// with the fewest total conflicts against remaining branches.
// This is a pure function with no git dependency.
func PlanMergeOrder(overlaps []FileOverlap, branches []string) []MergeStep {
	if len(branches) == 0 {
		return nil
	}

	idx := make(map[string]int, len(branches))
	for i, b := range branches {
		idx[b] = i
	}

	matrix := buildConflictMatrix(overlaps, idx, len(branches))
	return selectMergeOrder(matrix, branches)
}

// buildConflictMatrix builds an NxN matrix counting file overlaps between branch pairs.
func buildConflictMatrix(overlaps []FileOverlap, idx map[string]int, n int) [][]int {
	matrix := make([][]int, n)
	for i := range n {
		matrix[i] = make([]int, n)
	}

	for _, o := range overlaps {
		for a := range len(o.Branches) {
			for b := a + 1; b < len(o.Branches); b++ {
				ia, okA := idx[o.Branches[a]]
				ib, okB := idx[o.Branches[b]]
				if okA && okB {
					matrix[ia][ib]++
					matrix[ib][ia]++
				}
			}
		}
	}
	return matrix
}

// selectMergeOrder greedily picks branches with fewest conflicts against remaining branches.
func selectMergeOrder(matrix [][]int, branches []string) []MergeStep {
	n := len(branches)
	remaining := make([]int, n)
	for i := range n {
		remaining[i] = i
	}

	steps := make([]MergeStep, 0, n)
	for order := 1; len(remaining) > 0; order++ {
		bestIdx := 0
		bestConflicts := totalConflicts(matrix, remaining[0], remaining)

		for i := 1; i < len(remaining); i++ {
			c := totalConflicts(matrix, remaining[i], remaining)
			if c < bestConflicts || (c == bestConflicts && branches[remaining[i]] < branches[remaining[bestIdx]]) {
				bestIdx = i
				bestConflicts = c
			}
		}

		picked := remaining[bestIdx]
		steps = append(steps, MergeStep{
			Order:     order,
			Branch:    branches[picked],
			Conflicts: bestConflicts,
		})

		remaining = slices.Delete(remaining, bestIdx, bestIdx+1)
	}

	return steps
}

// totalConflicts computes the sum of conflicts between branchIdx and all other remaining branches.
func totalConflicts(matrix [][]int, branchIdx int, remaining []int) int {
	total := 0
	for _, r := range remaining {
		if r != branchIdx {
			total += matrix[branchIdx][r]
		}
	}
	return total
}
