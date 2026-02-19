package conflict

import (
	"errors"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/resolver"
)

// mockRunner implements git.Runner for testing within the conflict package.
type mockRunner struct {
	run func(args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	return m.run(args...)
}

func (m *mockRunner) RunInDir(_ string, _ ...string) (string, error) {
	return "", nil
}

func TestDetectOverlapsNoOverlap(t *testing.T) {
	diffs := map[string][]string{
		"feature/a": {"file1.go"},
		"feature/b": {"file2.go"},
	}

	result := DetectOverlaps(diffs)
	if len(result.Overlaps) != 0 {
		t.Errorf("expected no overlaps, got %d", len(result.Overlaps))
	}
	if result.TotalBranches != 2 {
		t.Errorf("TotalBranches = %d, want 2", result.TotalBranches)
	}
	if result.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", result.TotalFiles)
	}
}

func TestDetectOverlapsLowSeverity(t *testing.T) {
	diffs := map[string][]string{
		"feature/a": {"shared.go", "a-only.go"},
		"feature/b": {"shared.go", "b-only.go"},
	}

	result := DetectOverlaps(diffs)
	if len(result.Overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(result.Overlaps))
	}
	o := result.Overlaps[0]
	if o.File != "shared.go" {
		t.Errorf("File = %q, want %q", o.File, "shared.go")
	}
	if o.Severity != SeverityLow {
		t.Errorf("Severity = %q, want %q", o.Severity, SeverityLow)
	}
	if len(o.Branches) != 2 {
		t.Errorf("Branches count = %d, want 2", len(o.Branches))
	}
}

func TestDetectOverlapsHighSeverity(t *testing.T) {
	diffs := map[string][]string{
		"feature/a": {"shared.go"},
		"feature/b": {"shared.go"},
		"feature/c": {"shared.go"},
	}

	result := DetectOverlaps(diffs)
	if len(result.Overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(result.Overlaps))
	}
	o := result.Overlaps[0]
	if o.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want %q", o.Severity, SeverityHigh)
	}
	if len(o.Branches) != 3 {
		t.Errorf("Branches count = %d, want 3", len(o.Branches))
	}
}

func TestDetectOverlapsSortOrder(t *testing.T) {
	diffs := map[string][]string{
		"feature/a": {"low.go", "high.go"},
		"feature/b": {"low.go", "high.go"},
		"feature/c": {"high.go"},
	}

	result := DetectOverlaps(diffs)
	if len(result.Overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %d", len(result.Overlaps))
	}
	// high severity should come first
	if result.Overlaps[0].Severity != SeverityHigh {
		t.Errorf("first overlap should be high severity, got %q", result.Overlaps[0].Severity)
	}
	if result.Overlaps[1].Severity != SeverityLow {
		t.Errorf("second overlap should be low severity, got %q", result.Overlaps[1].Severity)
	}
}

func TestDetectOverlapsAlphabeticalWithinSeverity(t *testing.T) {
	diffs := map[string][]string{
		"feature/a": {"z.go", "a.go"},
		"feature/b": {"z.go", "a.go"},
	}

	result := DetectOverlaps(diffs)
	if len(result.Overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %d", len(result.Overlaps))
	}
	if result.Overlaps[0].File != "a.go" {
		t.Errorf("first overlap file = %q, want %q", result.Overlaps[0].File, "a.go")
	}
	if result.Overlaps[1].File != "z.go" {
		t.Errorf("second overlap file = %q, want %q", result.Overlaps[1].File, "z.go")
	}
}

func TestDetectOverlapsEmpty(t *testing.T) {
	result := DetectOverlaps(map[string][]string{})
	if len(result.Overlaps) != 0 {
		t.Errorf("expected no overlaps, got %d", len(result.Overlaps))
	}
	if result.TotalBranches != 0 {
		t.Errorf("TotalBranches = %d, want 0", result.TotalBranches)
	}
}

func TestDetectOverlapsBranchesSorted(t *testing.T) {
	diffs := map[string][]string{
		"feature/z": {"shared.go"},
		"feature/a": {"shared.go"},
		"feature/m": {"shared.go"},
	}

	result := DetectOverlaps(diffs)
	if len(result.Overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(result.Overlaps))
	}
	branches := result.Overlaps[0].Branches
	if branches[0] != "feature/a" || branches[1] != "feature/m" || branches[2] != "feature/z" {
		t.Errorf("branches not sorted: %v", branches)
	}
}

func TestSeverityLabel(t *testing.T) {
	tests := []struct {
		name string
		o    FileOverlap
		want string
	}{
		{
			name: "low with 2 branches",
			o:    FileOverlap{Severity: SeverityLow, Branches: []string{"a", "b"}},
			want: "low (2)",
		},
		{
			name: "high with 3 branches",
			o:    FileOverlap{Severity: SeverityHigh, Branches: []string{"a", "b", "c"}},
			want: "high (3)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SeverityLabel(tt.o)
			if got != tt.want {
				t.Errorf("SeverityLabel = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectDiffsSuccess(t *testing.T) {
	r := &mockRunner{
		run: func(args ...string) (string, error) {
			// args: diff --name-only main...feature/x
			if args[0] == "diff" && strings.Contains(args[2], "feature/a") {
				return "file-a.go\nshared.go", nil
			}
			if args[0] == "diff" && strings.Contains(args[2], "feature/b") {
				return "file-b.go\nshared.go", nil
			}
			return "", nil
		},
	}

	branches := []resolver.WorktreeInfo{
		{Branch: "feature/a", Path: "/wt/a"},
		{Branch: "feature/b", Path: "/wt/b"},
	}

	diffs, err := CollectDiffs(r, "main", branches)
	if err != nil {
		t.Fatalf("CollectDiffs: %v", err)
	}
	if len(diffs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(diffs))
	}
	if len(diffs["feature/a"]) != 2 {
		t.Errorf("feature/a files = %d, want 2", len(diffs["feature/a"]))
	}
	if len(diffs["feature/b"]) != 2 {
		t.Errorf("feature/b files = %d, want 2", len(diffs["feature/b"]))
	}
}

func TestCollectDiffsEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", nil
		},
	}

	diffs, err := CollectDiffs(r, "main", nil)
	if err != nil {
		t.Fatalf("CollectDiffs: %v", err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected empty diffs, got %d", len(diffs))
	}
}

func TestCollectDiffsError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New("git diff failed")
		},
	}

	branches := []resolver.WorktreeInfo{
		{Branch: "feature/a", Path: "/wt/a"},
	}

	_, err := CollectDiffs(r, "main", branches)
	if err == nil {
		t.Fatal("expected error from CollectDiffs")
	}
	if !strings.Contains(err.Error(), "diff feature/a") {
		t.Errorf("error = %q, want to contain branch name", err.Error())
	}
}

func TestDryMergeAllClean(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "abc123", nil // clean merge
		},
	}

	branches := []resolver.WorktreeInfo{
		{Branch: "feature/a", Path: "/wt/a"},
		{Branch: "feature/b", Path: "/wt/b"},
	}

	results, err := DryMergeAll(r, branches)
	if err != nil {
		t.Fatalf("DryMergeAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 pair result, got %d", len(results))
	}
	if results[0].HasConflicts {
		t.Error("expected no conflicts")
	}
	if results[0].Branch1 != "feature/a" || results[0].Branch2 != "feature/b" {
		t.Errorf("branches = %q/%q, want feature/a/feature/b", results[0].Branch1, results[0].Branch2)
	}
}

func TestDryMergeAllConflicts(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "abc123\nCONFLICT (content): Merge conflict in shared.go", errors.New("exit status 1")
		},
	}

	branches := []resolver.WorktreeInfo{
		{Branch: "feature/a", Path: "/wt/a"},
		{Branch: "feature/b", Path: "/wt/b"},
	}

	results, err := DryMergeAll(r, branches)
	if err != nil {
		t.Fatalf("DryMergeAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 pair result, got %d", len(results))
	}
	if !results[0].HasConflicts {
		t.Error("expected conflicts")
	}
	if len(results[0].ConflictFiles) != 1 || results[0].ConflictFiles[0] != "shared.go" {
		t.Errorf("ConflictFiles = %v, want [shared.go]", results[0].ConflictFiles)
	}
}

func TestDryMergeAllError(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", errors.New("not a valid object")
		},
	}

	branches := []resolver.WorktreeInfo{
		{Branch: "feature/a", Path: "/wt/a"},
		{Branch: "feature/b", Path: "/wt/b"},
	}

	_, err := DryMergeAll(r, branches)
	if err == nil {
		t.Fatal("expected error from DryMergeAll")
	}
}

func TestDryMergeAllThreeBranches(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "abc123", nil
		},
	}

	branches := []resolver.WorktreeInfo{
		{Branch: "feature/a", Path: "/wt/a"},
		{Branch: "feature/b", Path: "/wt/b"},
		{Branch: "feature/c", Path: "/wt/c"},
	}

	results, err := DryMergeAll(r, branches)
	if err != nil {
		t.Fatalf("DryMergeAll: %v", err)
	}
	// 3 branches = 3 pairs: (a,b), (a,c), (b,c)
	if len(results) != 3 {
		t.Fatalf("expected 3 pair results, got %d", len(results))
	}
}

func TestDetectOverlapsMixedSeveritySortBothDirections(t *testing.T) {
	// With 4+ overlaps of mixed severity, the sort algorithm calls the
	// comparator with a=low, b=high (triggering `return 1` at line 83).
	diffs := map[string][]string{
		"feature/a": {"alpha.go", "beta.go", "gamma.go", "delta.go"},
		"feature/b": {"alpha.go", "beta.go", "gamma.go", "delta.go"},
		"feature/c": {"alpha.go", "gamma.go"},
	}

	result := DetectOverlaps(diffs)
	// alpha.go: 3 branches → high, beta.go: 2 → low, gamma.go: 3 → high, delta.go: 2 → low
	if len(result.Overlaps) != 4 {
		t.Fatalf("expected 4 overlaps, got %d", len(result.Overlaps))
	}
	// High severity should come first
	for i, o := range result.Overlaps {
		if i < 2 && o.Severity != SeverityHigh {
			t.Errorf("overlap[%d] severity = %q, want high", i, o.Severity)
		}
		if i >= 2 && o.Severity != SeverityLow {
			t.Errorf("overlap[%d] severity = %q, want low", i, o.Severity)
		}
	}
}

func TestDryMergeAllEmpty(t *testing.T) {
	r := &mockRunner{
		run: func(_ ...string) (string, error) {
			return "", nil
		},
	}

	results, err := DryMergeAll(r, nil)
	if err != nil {
		t.Fatalf("DryMergeAll: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}
