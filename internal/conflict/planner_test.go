package conflict

import "testing"

func TestPlanNoOverlaps(t *testing.T) {
	plan := Plan(&Analysis{})
	if len(plan.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(plan.Steps))
	}
}

func TestPlanNil(t *testing.T) {
	plan := Plan(nil)
	if len(plan.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(plan.Steps))
	}
}

func TestPlanSinglePair(t *testing.T) {
	analysis := &Analysis{
		Pairs: []PairConflict{
			{
				BranchA:      branchA,
				BranchB:      branchB,
				OverlapFiles: []string{fileShared},
			},
		},
	}

	plan := Plan(analysis)

	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.Steps))
	}

	// Both have the same overlap count (1), so alphabetical order
	if plan.Steps[0].Branch != branchA {
		t.Errorf("expected first step to be %s, got %s", branchA, plan.Steps[0].Branch)
	}
	if plan.Steps[1].Branch != branchB {
		t.Errorf("expected second step to be %s, got %s", branchB, plan.Steps[1].Branch)
	}
}

func TestPlanGreedyOrder(t *testing.T) {
	// Branch C overlaps with both A and B, while A and B don't overlap.
	// C should be merged last (most overlaps).
	analysis := &Analysis{
		Pairs: []PairConflict{
			{
				BranchA:      branchA,
				BranchB:      branchC,
				OverlapFiles: []string{"shared1.go"},
			},
			{
				BranchA:      branchB,
				BranchB:      branchC,
				OverlapFiles: []string{"shared2.go"},
			},
		},
	}

	plan := Plan(analysis)

	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}

	// A and B have 1 overlap each, C has 2 — so A and B first, then C
	if plan.Steps[2].Branch != branchC {
		t.Errorf("expected last step to be %s (most overlaps), got %s", branchC, plan.Steps[2].Branch)
	}
}
