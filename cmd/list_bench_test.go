package cmd

import (
	"strings"
	"sync"
	"testing"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

const benchFilterType = "bugfix"

// benchMockRunner simulates git status calls with minimal overhead.
type benchMockRunner struct{}

func (m *benchMockRunner) Run(_ ...string) (string, error) { return "", nil }
func (m *benchMockRunner) RunInDir(_ string, args ...string) (string, error) {
	// Simulate IsDirty returning clean, AheadBehind returning 0/0
	if len(args) > 0 && args[0] == "rev-list" {
		return "0\t0", nil
	}
	return "", nil
}

func makeBenchEntries(n int) []git.WorktreeEntry { //nolint:unparam // n is parameterized for benchmark flexibility
	prefixes := []string{"feature/", "bugfix/", "hotfix/"}
	entries := make([]git.WorktreeEntry, n)
	for i := range n {
		p := prefixes[i%len(prefixes)]
		entries[i] = git.WorktreeEntry{
			Path:   "/tmp/wt-" + string(rune('a'+i)),
			Branch: p + "task-" + string(rune('a'+i)),
		}
	}
	return entries
}

func BenchmarkListStatusCollectionSequential(b *testing.B) {
	r := &benchMockRunner{}
	entries := makeBenchEntries(10)
	prefixes := resolver.AllPrefixes()

	b.ResetTimer()
	for b.Loop() {
		rows := make([]resolver.WorktreeDetail, 0, len(entries))
		for _, e := range entries {
			var status resolver.WorktreeStatus
			if dirty, err := git.IsDirty(r, e.Path); err == nil && dirty {
				status.Dirty = true
			}
			ahead, behind, _ := git.AheadBehind(r, e.Path)
			status.Ahead = ahead
			status.Behind = behind
			rows = append(rows, resolver.NewWorktreeDetail(e.Branch, prefixes, e.Path, status, false))
		}
		_ = rows
	}
}

func BenchmarkListStatusCollectionParallel(b *testing.B) {
	r := &benchMockRunner{}
	entries := makeBenchEntries(10)
	prefixes := resolver.AllPrefixes()

	b.ResetTimer()
	for b.Loop() {
		rows := make([]resolver.WorktreeDetail, len(entries))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)

		for i, e := range entries {
			wg.Add(1)
			go func(idx int, e git.WorktreeEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				var status resolver.WorktreeStatus
				if dirty, err := git.IsDirty(r, e.Path); err == nil && dirty {
					status.Dirty = true
				}
				ahead, behind, _ := git.AheadBehind(r, e.Path)
				status.Ahead = ahead
				status.Behind = behind
				rows[idx] = resolver.NewWorktreeDetail(e.Branch, prefixes, e.Path, status, false)
			}(i, e)
		}
		wg.Wait()
		_ = rows
	}
}

func BenchmarkListWithTypeFilter(b *testing.B) {
	r := &benchMockRunner{}
	entries := makeBenchEntries(10)
	prefixes := resolver.AllPrefixes()
	filterType := benchFilterType

	b.ResetTimer()
	for b.Loop() {
		// Early filter
		var filtered []git.WorktreeEntry
		for _, e := range entries {
			_, matchedPrefix := resolver.TaskFromBranch(e.Branch, prefixes)
			entryType := strings.TrimSuffix(matchedPrefix, "/")
			if entryType != filterType {
				continue
			}
			filtered = append(filtered, e)
		}

		// Collect status only for matching
		rows := make([]resolver.WorktreeDetail, len(filtered))
		for i, e := range filtered {
			var status resolver.WorktreeStatus
			if dirty, err := git.IsDirty(r, e.Path); err == nil && dirty {
				status.Dirty = true
			}
			ahead, behind, _ := git.AheadBehind(r, e.Path)
			status.Ahead = ahead
			status.Behind = behind
			rows[i] = resolver.NewWorktreeDetail(e.Branch, prefixes, e.Path, status, false)
		}
		_ = rows
	}
}

func BenchmarkListWithoutTypeFilter(b *testing.B) {
	r := &benchMockRunner{}
	entries := makeBenchEntries(10)
	prefixes := resolver.AllPrefixes()

	b.ResetTimer()
	for b.Loop() {
		// Collect status for all entries (no early filter)
		rows := make([]resolver.WorktreeDetail, len(entries))
		for i, e := range entries {
			var status resolver.WorktreeStatus
			if dirty, err := git.IsDirty(r, e.Path); err == nil && dirty {
				status.Dirty = true
			}
			ahead, behind, _ := git.AheadBehind(r, e.Path)
			status.Ahead = ahead
			status.Behind = behind
			rows[i] = resolver.NewWorktreeDetail(e.Branch, prefixes, e.Path, status, false)
		}

		// Post-filter by type
		filtered := rows[:0]
		for _, row := range rows {
			if row.Type != benchFilterType {
				continue
			}
			filtered = append(filtered, row)
		}
		_ = filtered
	}
}
