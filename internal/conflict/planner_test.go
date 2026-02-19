package conflict

import (
	"testing"
)

const (
	branchA = "feature/a"
	branchB = "feature/b"
	branchC = "feature/c"
)

func TestPlanMergeOrderEmpty(t *testing.T) {
	steps := PlanMergeOrder(nil, nil)
	if len(steps) != 0 {
		t.Errorf("expected no steps, got %d", len(steps))
	}
}

func TestPlanMergeOrderSingleBranch(t *testing.T) {
	steps := PlanMergeOrder(nil, []string{branchA})
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Branch != branchA {
		t.Errorf("Branch = %q, want %q", steps[0].Branch, branchA)
	}
	if steps[0].Conflicts != 0 {
		t.Errorf("Conflicts = %d, want 0", steps[0].Conflicts)
	}
	if steps[0].Order != 1 {
		t.Errorf("Order = %d, want 1", steps[0].Order)
	}
}

func TestPlanMergeOrderNoOverlaps(t *testing.T) {
	branches := []string{branchA, branchB, branchC}
	steps := PlanMergeOrder(nil, branches)
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	// All should have 0 conflicts
	for _, s := range steps {
		if s.Conflicts != 0 {
			t.Errorf("Branch %q has %d conflicts, want 0", s.Branch, s.Conflicts)
		}
	}
	// With no conflicts, order should be alphabetical
	if steps[0].Branch != branchA {
		t.Errorf("first step = %q, want feature/a (alphabetical)", steps[0].Branch)
	}
}

func TestPlanMergeOrderThreeBranches(t *testing.T) {
	// Scenario: A and B share 2 files, A and C share 1 file, B and C share 0 files
	// Expected: C first (1 conflict), then A (tie-break alphabetical), then B (last)
	overlaps := []FileOverlap{
		{File: "shared1.go", Branches: []string{branchA, branchB}},
		{File: "shared2.go", Branches: []string{branchA, branchB}},
		{File: "shared3.go", Branches: []string{branchA, branchC}},
	}
	branches := []string{branchA, branchB, branchC}

	steps := PlanMergeOrder(overlaps, branches)
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}

	// B has 2 conflicts (with A), C has 1 (with A), A has 3 (with both)
	// → C should be first (fewest conflicts = 1)
	if steps[0].Branch != branchC {
		t.Errorf("first merge should be feature/c (fewest conflicts), got %q", steps[0].Branch)
	}

	// After removing C: A has 2 (with B), B has 2 (with A) → alphabetical → A
	if steps[1].Branch != branchA {
		t.Errorf("second merge should be feature/a, got %q", steps[1].Branch)
	}

	// Last remaining
	if steps[2].Branch != branchB {
		t.Errorf("third merge should be feature/b, got %q", steps[2].Branch)
	}
}

func TestPlanMergeOrderAlphabeticalTieBreak(t *testing.T) {
	// Both branches have same conflict count → alphabetical order wins
	overlaps := []FileOverlap{
		{File: "shared.go", Branches: []string{branchB, branchA}},
	}
	branches := []string{branchB, branchA}

	steps := PlanMergeOrder(overlaps, branches)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	// Both have 1 conflict with each other, alphabetical wins
	if steps[0].Branch != branchA {
		t.Errorf("first = %q, want feature/a (alphabetical tie-break)", steps[0].Branch)
	}
}

func TestPlanMergeOrderSequentialOrdering(t *testing.T) {
	branches := []string{branchA, branchB}
	steps := PlanMergeOrder(nil, branches)

	for i, s := range steps {
		if s.Order != i+1 {
			t.Errorf("step %d Order = %d, want %d", i, s.Order, i+1)
		}
	}
}
