package conflict

import (
	"sort"

	"github.com/lugassawan/rimba/internal/git"
)

// FileOverlap represents a file modified by multiple branches.
type FileOverlap struct {
	File     string
	Branches []string
}

// PairConflict represents the overlap between two branches.
type PairConflict struct {
	BranchA      string
	BranchB      string
	OverlapFiles []string
	HasConflict  bool // only set when dry-merge is used
}

// Analysis holds the results of a cross-worktree conflict analysis.
type Analysis struct {
	Overlaps []FileOverlap
	Pairs    []PairConflict
}

// Analyze detects file-level overlaps (and optionally real merge conflicts)
// between branches relative to the given base.
func Analyze(r git.Runner, base string, branches []string, dryMerge bool) (*Analysis, error) {
	branchFiles, err := collectBranchFiles(r, base, branches)
	if err != nil {
		return nil, err
	}

	overlaps := buildFileOverlaps(branchFiles)
	pairs := computePairConflicts(r, branches, branchFiles, dryMerge)

	return &Analysis{Overlaps: overlaps, Pairs: pairs}, nil
}

// collectBranchFiles returns the changed files for each branch relative to base.
func collectBranchFiles(r git.Runner, base string, branches []string) (map[string][]string, error) {
	branchFiles := make(map[string][]string, len(branches))
	for _, b := range branches {
		files, err := git.DiffNameOnly(r, base, b)
		if err != nil {
			return nil, err
		}
		branchFiles[b] = files
	}
	return branchFiles, nil
}

// buildFileOverlaps finds files touched by 2+ branches.
func buildFileOverlaps(branchFiles map[string][]string) []FileOverlap {
	fileIndex := make(map[string][]string)
	for b, files := range branchFiles {
		for _, f := range files {
			fileIndex[f] = append(fileIndex[f], b)
		}
	}

	var overlaps []FileOverlap
	for f, bs := range fileIndex {
		if len(bs) < 2 {
			continue
		}
		sort.Strings(bs)
		overlaps = append(overlaps, FileOverlap{File: f, Branches: bs})
	}
	sort.Slice(overlaps, func(i, j int) bool {
		return overlaps[i].File < overlaps[j].File
	})
	return overlaps
}

// computePairConflicts checks each branch pair for shared files and optional dry-merge conflicts.
func computePairConflicts(r git.Runner, branches []string, branchFiles map[string][]string, dryMerge bool) []PairConflict {
	var pairs []PairConflict
	for i := range len(branches) {
		for j := i + 1; j < len(branches); j++ {
			a, b := branches[i], branches[j]
			common := intersect(branchFiles[a], branchFiles[b])
			if len(common) == 0 {
				continue
			}

			pc := PairConflict{BranchA: a, BranchB: b, OverlapFiles: common}
			if dryMerge {
				checkDryMerge(r, a, b, &pc)
			}
			pairs = append(pairs, pc)
		}
	}
	return pairs
}

// checkDryMerge performs an in-memory merge to detect real conflicts.
func checkDryMerge(r git.Runner, a, b string, pc *PairConflict) {
	mergeBase, err := git.MergeBase(r, a, b)
	if err != nil {
		return
	}
	hasConflict, err := git.MergeTree(r, mergeBase, a, b)
	if err != nil {
		return
	}
	pc.HasConflict = hasConflict
}

func intersect(a, b []string) []string {
	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[s] = struct{}{}
	}

	var result []string
	for _, s := range b {
		if _, ok := set[s]; ok {
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result
}
