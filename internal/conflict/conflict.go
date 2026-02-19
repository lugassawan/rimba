package conflict

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// Severity indicates how critical a file overlap is.
type Severity string

const (
	SeverityLow  Severity = "low"
	SeverityHigh Severity = "high"
)

// FileOverlap represents a file that is modified in multiple branches.
type FileOverlap struct {
	File     string
	Branches []string
	Severity Severity
}

// CheckResult holds the outcome of an overlap detection.
type CheckResult struct {
	Overlaps      []FileOverlap
	TotalFiles    int
	TotalBranches int
}

// DryMergeResult holds the outcome of a simulated merge between two branches.
type DryMergeResult struct {
	Branch1       string
	Branch2       string
	HasConflicts  bool
	ConflictFiles []string
}

// DetectOverlaps analyzes a map of branch→files and returns files modified in 2+ branches.
// This is a pure function with no git dependency.
// Results are sorted: high severity first, then alphabetically by file name.
func DetectOverlaps(diffs map[string][]string) *CheckResult {
	result := &CheckResult{TotalBranches: len(diffs)}

	// Build file → branches map
	fileMap := make(map[string][]string)
	allFiles := make(map[string]struct{})
	for branch, files := range diffs {
		for _, f := range files {
			allFiles[f] = struct{}{}
			fileMap[f] = append(fileMap[f], branch)
		}
	}
	result.TotalFiles = len(allFiles)

	// Collect files with 2+ branches
	for file, branches := range fileMap {
		if len(branches) < 2 {
			continue
		}
		slices.Sort(branches)
		sev := SeverityLow
		if len(branches) >= 3 {
			sev = SeverityHigh
		}
		result.Overlaps = append(result.Overlaps, FileOverlap{
			File:     file,
			Branches: branches,
			Severity: sev,
		})
	}

	// Sort: high severity first, then alphabetically
	slices.SortFunc(result.Overlaps, func(a, b FileOverlap) int {
		if a.Severity != b.Severity {
			if a.Severity == SeverityHigh {
				return -1
			}
			return 1
		}
		return strings.Compare(a.File, b.File)
	})

	return result
}

// CollectDiffs runs git diff --name-only for each branch vs mainBranch, in parallel.
func CollectDiffs(r git.Runner, mainBranch string, branches []resolver.WorktreeInfo) (map[string][]string, error) {
	diffs := make(map[string][]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	sem := make(chan struct{}, 8)

	for _, wt := range branches {
		wg.Add(1)
		go func(wt resolver.WorktreeInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			files, err := git.DiffNameOnly(r, mainBranch, wt.Branch)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("diff %s: %w", wt.Branch, err)
				}
				return
			}
			diffs[wt.Branch] = files
		}(wt)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return diffs, nil
}

// DryMergeAll runs git merge-tree for all unique branch pairs, in parallel.
func DryMergeAll(r git.Runner, branches []resolver.WorktreeInfo) ([]DryMergeResult, error) {
	type pair struct{ i, j int }
	var pairs []pair
	for i := range branches {
		for j := i + 1; j < len(branches); j++ {
			pairs = append(pairs, pair{i, j})
		}
	}

	results := make([]DryMergeResult, len(pairs))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	sem := make(chan struct{}, 4)

	for idx, p := range pairs {
		wg.Add(1)
		go func(idx int, p pair) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			mt, err := git.MergeTree(r, branches[p.i].Branch, branches[p.j].Branch)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("merge-tree %s %s: %w", branches[p.i].Branch, branches[p.j].Branch, err)
				}
				return
			}
			results[idx] = DryMergeResult{
				Branch1:       branches[p.i].Branch,
				Branch2:       branches[p.j].Branch,
				HasConflicts:  mt.HasConflicts,
				ConflictFiles: mt.ConflictFiles,
			}
		}(idx, p)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// SeverityLabel returns a display string for a file overlap, e.g. "high (3)".
func SeverityLabel(o FileOverlap) string {
	return fmt.Sprintf("%s (%d)", o.Severity, len(o.Branches))
}
