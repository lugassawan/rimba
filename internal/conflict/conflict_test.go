package conflict

import (
	"fmt"
	"strings"
	"testing"
)

const (
	diffRefA      = "main...feature/a"
	diffRefB      = "main...feature/b"
	branchA       = "feature/a"
	branchB       = "feature/b"
	branchC       = "feature/c"
	errUnexpected = "unexpected error: %v"
	fileShared    = "shared.go"
)

// mockRunner implements git.Runner for testing.
type mockRunner struct {
	diffResults   map[string][]string // "base...branch" → files
	mergeBase     string
	mergeConflict bool
}

func (m *mockRunner) Run(args ...string) (string, error) {
	cmd := strings.Join(args, " ")

	// Handle: git diff --name-only base...branch
	if len(args) >= 3 && args[0] == "diff" && args[1] == "--name-only" {
		ref := args[2]
		if files, ok := m.diffResults[ref]; ok {
			return strings.Join(files, "\n"), nil
		}
		return "", nil
	}

	// Handle: git merge-base branch1 branch2
	if len(args) >= 3 && args[0] == "merge-base" {
		return m.mergeBase, nil
	}

	// Handle: git merge-tree --write-tree ...
	if len(args) >= 1 && args[0] == "merge-tree" {
		if m.mergeConflict {
			return "CONFLICT (content): Merge conflict in file.go", fmt.Errorf("merge conflict: %w", fmt.Errorf("exit 1"))
		}
		return "abc123", nil
	}

	return "", fmt.Errorf("unexpected command: %s", cmd)
}

func (m *mockRunner) RunInDir(_ string, args ...string) (string, error) {
	return m.Run(args...)
}

func TestAnalyzeNoOverlap(t *testing.T) {
	r := &mockRunner{
		diffResults: map[string][]string{
			diffRefA: {"a.go"},
			diffRefB: {"b.go"},
		},
	}

	analysis, err := Analyze(r, "main", []string{branchA, branchB}, false)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}

	if len(analysis.Overlaps) != 0 {
		t.Errorf("expected 0 overlaps, got %d", len(analysis.Overlaps))
	}
	if len(analysis.Pairs) != 0 {
		t.Errorf("expected 0 pairs, got %d", len(analysis.Pairs))
	}
}

func TestAnalyzeWithOverlap(t *testing.T) {
	r := &mockRunner{
		diffResults: map[string][]string{
			diffRefA: {fileShared, "a.go"},
			diffRefB: {fileShared, "b.go"},
		},
	}

	analysis, err := Analyze(r, "main", []string{branchA, branchB}, false)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}

	if len(analysis.Overlaps) != 1 {
		t.Fatalf("expected 1 overlap, got %d", len(analysis.Overlaps))
	}
	if analysis.Overlaps[0].File != fileShared {
		t.Errorf("expected overlap on %s, got %s", fileShared, analysis.Overlaps[0].File)
	}

	if len(analysis.Pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(analysis.Pairs))
	}
	if len(analysis.Pairs[0].OverlapFiles) != 1 {
		t.Errorf("expected 1 overlap file, got %d", len(analysis.Pairs[0].OverlapFiles))
	}
}

func TestAnalyzeWithDryMerge(t *testing.T) {
	r := &mockRunner{
		diffResults: map[string][]string{
			diffRefA: {fileShared},
			diffRefB: {fileShared},
		},
		mergeBase:     "abc123",
		mergeConflict: true,
	}

	analysis, err := Analyze(r, "main", []string{branchA, branchB}, true)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}

	if len(analysis.Pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(analysis.Pairs))
	}
	if !analysis.Pairs[0].HasConflict {
		t.Error("expected HasConflict to be true")
	}
}

func TestAnalyzeEmptyBranches(t *testing.T) {
	r := &mockRunner{diffResults: map[string][]string{}}

	analysis, err := Analyze(r, "main", nil, false)
	if err != nil {
		t.Fatalf(errUnexpected, err)
	}

	if len(analysis.Overlaps) != 0 {
		t.Errorf("expected 0 overlaps, got %d", len(analysis.Overlaps))
	}
}

func TestIntersect(t *testing.T) {
	result := intersect([]string{"a", "b", "c"}, []string{"b", "c", "d"})
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0] != "b" || result[1] != "c" {
		t.Errorf("expected [b, c], got %v", result)
	}
}

func TestIntersectEmpty(t *testing.T) {
	result := intersect([]string{"a"}, []string{"b"})
	if len(result) != 0 {
		t.Errorf("expected 0 items, got %d", len(result))
	}
}
