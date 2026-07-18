package metrics

import (
	"encoding/json"
	"math"
	"slices"
	"sort"
)

// ReadRuns reads all Run records from path in file order (oldest first).
// Returns (nil, nil) if path does not exist — mirrors config's absent-file
// contract, not an error case. A present-but-empty file also returns (nil, nil).
func ReadRuns(path string) ([]Run, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	if lines == nil {
		return nil, nil
	}

	runs := make([]Run, 0, len(lines))
	for _, line := range lines {
		var run Run
		if err := json.Unmarshal([]byte(line), &run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// PhaseStat holds count/percentile/mean duration statistics for one named
// span within a command's aggregated runs.
type PhaseStat struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	P50MS  int64  `json:"p50_ms"`
	P95MS  int64  `json:"p95_ms"`
	MeanMS int64  `json:"mean_ms"`
}

// CommandReport holds the aggregated phase statistics for one Command across
// all runs matching it.
type CommandReport struct {
	Command string      `json:"command"`
	Count   int         `json:"count"`
	Phases  []PhaseStat `json:"phases"`
}

// Aggregate groups runs by Command and computes per-phase (span name) count/
// p50/p95/mean duration_ms. Phases within a command are sorted by name for
// deterministic output. Commands are sorted by name. Does not mutate runs'
// order — grouping is done into freshly-built maps/slices.
func Aggregate(runs []Run) []CommandReport {
	commandCounts := map[string]int{}
	phasesByCommand := map[string]map[string][]int64{}

	for _, run := range runs {
		commandCounts[run.Command]++
		phases, ok := phasesByCommand[run.Command]
		if !ok {
			phases = map[string][]int64{}
			phasesByCommand[run.Command] = phases
		}
		for _, span := range run.Spans {
			phases[span.Name] = append(phases[span.Name], span.DurationMS)
		}
	}

	commands := make([]string, 0, len(commandCounts))
	for command := range commandCounts {
		commands = append(commands, command)
	}
	sort.Strings(commands)

	reports := make([]CommandReport, 0, len(commands))
	for _, command := range commands {
		reports = append(reports, CommandReport{
			Command: command,
			Count:   commandCounts[command],
			Phases:  buildPhaseStats(phasesByCommand[command]),
		})
	}
	return reports
}

// buildPhaseStats turns a phase-name → durations map into a name-sorted
// slice of PhaseStat.
func buildPhaseStats(durationsByPhase map[string][]int64) []PhaseStat {
	names := make([]string, 0, len(durationsByPhase))
	for name := range durationsByPhase {
		names = append(names, name)
	}
	sort.Strings(names)

	stats := make([]PhaseStat, 0, len(names))
	for _, name := range names {
		stats = append(stats, computePhaseStat(name, durationsByPhase[name]))
	}
	return stats
}

// computePhaseStat sorts a copy of durations and derives count/p50/p95/mean
// from it. Copying before sorting keeps the caller's map value untouched.
func computePhaseStat(name string, durations []int64) PhaseStat {
	sorted := make([]int64, len(durations))
	copy(sorted, durations)
	slices.Sort(sorted)

	return PhaseStat{
		Name:   name,
		Count:  len(sorted),
		P50MS:  percentile(sorted, 0.5),
		P95MS:  percentile(sorted, 0.95),
		MeanMS: mean(sorted),
	}
}

// percentile returns the nearest-rank percentile p (e.g. 0.5, 0.95) of an
// already-sorted-ascending slice of durations.
func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	idx = max(idx, 0)
	idx = min(idx, len(sorted)-1)
	return sorted[idx]
}

// mean returns the integer-truncated average of durations.
func mean(durations []int64) int64 {
	if len(durations) == 0 {
		return 0
	}
	var sum int64
	for _, d := range durations {
		sum += d
	}
	return sum / int64(len(durations))
}
