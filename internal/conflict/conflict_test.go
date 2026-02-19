package conflict

import (
	"testing"
)

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
