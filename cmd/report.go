package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/observability"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

// reportEnvelope reads just enough of a JSONL line to classify it before a
// full unmarshal: schema_version to detect drift, kind to route it to the
// right struct.
type reportEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
}

// reportPhaseKey groups span durations by (command, phase/module name).
type reportPhaseKey struct {
	Command string
	Name    string
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Aggregate this repo's observability metrics into per-command timing stats",
	Long: `Aggregates the current repo's .metrics.jsonl day-files into per-(command, phase)
count/p50/p95/mean duration stats, plus an environment header. The output is designed
to be copy-pasted directly into a filed GitHub issue.

Works even in a repo without .rimba/ initialized: it simply finds zero files and
reports "no data".`,
	Example: `  rimba report
  rimba report --json`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SetContext(withBestEffortConfig(cmd))
		ctx := cmd.Context()
		r := newRunner(ctx)

		repoRoot, err := git.MainRepoRoot(ctx, r)
		if err != nil {
			return err
		}

		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return err
		}

		data := collectReportData(cacheDir, repoRoot)

		if isJSON(cmd) {
			return output.WriteJSON(cmd.OutOrStdout(), version, "report", data)
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)
		renderReport(cmd.OutOrStdout(), p, data)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)
}

// collectReportData aggregates this repo's .metrics.jsonl day-files into
// per-(command, phase) stats and, since SpanRecord carries no version field
// (see internal/observability/record.go), separately scans .log.jsonl for
// the distinct rimba_version values seen.
func collectReportData(cacheDir, repoRoot string) output.ReportData {
	dir := filepath.Join(cacheDir, "rimba")
	prefix := observability.RepoPrefix(repoRoot)

	durations := make(map[reportPhaseKey][]int64)
	unparseable := 0

	metricsFiles := observability.ListDayFiles(dir, prefix, ".metrics.jsonl")
	for _, f := range metricsFiles {
		unparseable += scanMetricsFile(f, durations)
	}

	versionSet := make(map[string]struct{})
	logFiles := observability.ListDayFiles(dir, prefix, ".log.jsonl")
	for _, f := range logFiles {
		scanLogFileVersions(f, versionSet)
	}

	versions := make([]string, 0, len(versionSet))
	for v := range versionSet {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	return output.ReportData{
		Env: output.ReportEnvHeader{
			OS:                runtime.GOOS,
			Arch:              runtime.GOARCH,
			NumCPU:            runtime.NumCPU(),
			RimbaVersion:      version,
			RimbaVersionsSeen: versions,
			UnparseableLines:  unparseable,
		},
		Phases: buildPhaseStats(durations),
	}
}

// scanMetricsFile reads path line by line, folding each valid span's
// DurationMS into durations keyed by (Command, Name). The returned count is
// lines that failed to parse or carried a schema_version mismatch — both
// signal a possibly corrupted/interleaved write.
func scanMetricsFile(path string, durations map[reportPhaseKey][]int64) int {
	f, err := os.Open(path) //nolint:gosec // path comes from observability.ListDayFiles under the user's cache dir
	if err != nil {
		return 0
	}
	defer f.Close()

	unparseable := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var env reportEnvelope
		if err := json.Unmarshal(line, &env); err != nil || env.SchemaVersion != observability.SchemaVersion {
			unparseable++
			continue
		}
		if env.Kind != "span" {
			continue
		}
		var span observability.SpanRecord
		if err := json.Unmarshal(line, &span); err != nil {
			unparseable++
			continue
		}
		key := reportPhaseKey{Command: span.Command, Name: span.Name}
		durations[key] = append(durations[key], span.DurationMS)
	}
	return unparseable
}

// scanLogFileVersions reads path line by line, adding the RimbaVersion of
// every well-formed "command"-kind, current-schema line to seen.
func scanLogFileVersions(path string, seen map[string]struct{}) {
	f, err := os.Open(path) //nolint:gosec // path comes from observability.ListDayFiles under the user's cache dir
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec observability.CommandRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if rec.SchemaVersion != observability.SchemaVersion || rec.Kind != "command" || rec.RimbaVersion == "" {
			continue
		}
		seen[rec.RimbaVersion] = struct{}{}
	}
}

// buildPhaseStats computes count/p50/p95/mean per group, sorted by command
// then phase name for stable output. Always returns a non-nil (possibly
// empty) slice so JSON output serializes as [] rather than null.
func buildPhaseStats(durations map[reportPhaseKey][]int64) []output.ReportPhaseStats {
	phases := make([]output.ReportPhaseStats, 0, len(durations))
	for key, ds := range durations {
		phases = append(phases, output.ReportPhaseStats{
			Command: key.Command,
			Phase:   key.Name,
			Count:   len(ds),
			P50MS:   percentile(ds, 0.5),
			P95MS:   percentile(ds, 0.95),
			MeanMS:  mean(ds),
		})
	}
	sort.Slice(phases, func(i, j int) bool {
		if phases[i].Command != phases[j].Command {
			return phases[i].Command < phases[j].Command
		}
		return phases[i].Phase < phases[j].Phase
	})
	return phases
}

// percentile returns the p-th percentile (0 < p <= 1) of durations via a
// simple sort-and-index approach — adequate at this data scale, no need for
// a streaming/approximate algorithm.
func percentile(durations []int64, p float64) float64 {
	sorted := append([]int64(nil), durations...)
	slices.Sort(sorted)
	idx := int(float64(len(sorted)) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return float64(sorted[idx])
}

func mean(durations []int64) float64 {
	var sum int64
	for _, d := range durations {
		sum += d
	}
	return float64(sum) / float64(len(durations))
}

// renderReport writes the plain-text env header and phase table to out.
func renderReport(out io.Writer, p *termcolor.Painter, data output.ReportData) {
	fmt.Fprintf(out, "OS: %s  Arch: %s  CPUs: %d  Rimba: %s\n",
		data.Env.OS, data.Env.Arch, data.Env.NumCPU, data.Env.RimbaVersion)
	if len(data.Env.RimbaVersionsSeen) > 0 {
		fmt.Fprintf(out, "Versions seen in data: %s\n", strings.Join(data.Env.RimbaVersionsSeen, ", "))
	}
	fmt.Fprintf(out, "Unparseable lines: %d\n\n", data.Env.UnparseableLines)

	if len(data.Phases) == 0 {
		fmt.Fprintln(out, "No observability data found for this repo.")
		return
	}

	tbl := termcolor.NewTable(2)
	tbl.AddRow(
		p.Paint("COMMAND", termcolor.Bold),
		p.Paint("PHASE", termcolor.Bold),
		p.Paint("COUNT", termcolor.Bold),
		p.Paint("P50", termcolor.Bold),
		p.Paint("P95", termcolor.Bold),
		p.Paint("MEAN", termcolor.Bold),
	)
	for _, ph := range data.Phases {
		tbl.AddRow(
			"  "+ph.Command,
			ph.Phase,
			strconv.Itoa(ph.Count),
			formatMS(ph.P50MS),
			formatMS(ph.P95MS),
			formatMS(ph.MeanMS),
		)
	}
	tbl.Render(out)
}

// formatMS renders a millisecond duration to one decimal place.
func formatMS(ms float64) string {
	return fmt.Sprintf("%.1fms", ms)
}
