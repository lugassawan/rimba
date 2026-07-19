package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/spf13/cobra"
)

// addObservabilityProbeCmd registers a throwaway subcommand (deliberately not
// skipConfig-annotated, unlike version/status) so PersistentPreRunE runs its
// full path — including the observability build — when Execute() invokes it.
// Returns a cleanup func that removes it and resets rootCmd's execution state
// (mirroring TestExecute's cleanup).
func addObservabilityProbeCmd(t *testing.T) {
	t.Helper()
	probe := &cobra.Command{
		Use: "observability-probe",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	rootCmd.AddCommand(probe)
	rootCmd.SetArgs([]string{"observability-probe"})

	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.RemoveCommand(probe)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		// Execute() leaves rootCmd's context as the (now-cancelled) signal
		// context; reset so later tests get a live one (mirrors TestExecute).
		rootCmd.SetContext(context.Background())
		for _, c := range rootCmd.Commands() {
			if c.Name() == "help" {
				rootCmd.RemoveCommand(c)
			}
		}
	})
}

// redirectCacheDir points os.UserCacheDir() at a fresh temp dir for the
// duration of the test (mirrors internal/observability/sink_test.go's
// HOME-override pattern), so these tests never touch the real user cache dir.
func redirectCacheDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")
	os.Unsetenv("XDG_CACHE_HOME")
	return home
}

// findCacheJSONLFiles walks home looking for any observability day-file,
// wherever os.UserCacheDir() placed the "rimba" subdir on this platform.
func findCacheJSONLFiles(t *testing.T, home string) []string {
	t.Helper()
	var matches []string
	err := filepath.WalkDir(home, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".jsonl") && filepath.Base(filepath.Dir(path)) == "rimba" {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", home, err)
	}
	return matches
}

// jsonlRecords parses a JSONL file into a slice of generic records.
func jsonlRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("unmarshal jsonl line %q: %v", line, err)
		}
		records = append(records, m)
	}
	return records
}

// TestExecuteRecordsCommandAndRootSpanSharingRunID is the end-to-end check for
// cmd/root.go's observability wiring: PersistentPreRunE builds a Recorder,
// Execute()'s post-ExecuteContext defer finalizes it (via the lastRecorder
// package var — rootCmd.Context() does not reflect the invoked subcommand's
// SetContext; see lastRecorder's doc comment), producing a day-file with one
// CommandRecord and one root SpanRecord sharing a single run_id.
func TestExecuteRecordsCommandAndRootSpanSharingRunID(t *testing.T) {
	home := redirectCacheDir(t)

	dir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "../worktrees"}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})
	restore := overrideNewRunner(r)
	defer restore()

	addObservabilityProbeCmd(t)

	if err := Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	logFile, metricFile := splitDayFiles(t, findCacheJSONLFiles(t, home))

	cmdRecord := findRecord(jsonlRecords(t, logFile), "command", "", "")
	if cmdRecord == nil {
		t.Fatalf("expected a CommandRecord in %s", logFile)
	}
	if cmdRecord["outcome"] != "success" {
		t.Errorf("CommandRecord outcome = %v, want success", cmdRecord["outcome"])
	}

	rootSpan := findRecord(jsonlRecords(t, metricFile), "span", "name", "command")
	if rootSpan == nil {
		t.Fatalf("expected a root SpanRecord (name=command) in %s", metricFile)
	}

	runID, _ := cmdRecord["run_id"].(string)
	spanRunID, _ := rootSpan["run_id"].(string)
	if runID == "" || runID != spanRunID {
		t.Errorf("CommandRecord run_id = %q, root SpanRecord run_id = %q; want equal and non-empty", runID, spanRunID)
	}
}

// splitDayFiles splits files into the .log.jsonl and .metrics.jsonl paths,
// failing the test if either is missing.
func splitDayFiles(t *testing.T, files []string) (logFile, metricFile string) {
	t.Helper()
	for _, f := range files {
		switch {
		case strings.HasSuffix(f, ".log.jsonl"):
			logFile = f
		case strings.HasSuffix(f, ".metrics.jsonl"):
			metricFile = f
		}
	}
	if logFile == "" {
		t.Fatalf("expected a .log.jsonl file, found: %v", files)
	}
	if metricFile == "" {
		t.Fatalf("expected a .metrics.jsonl file, found: %v", files)
	}
	return logFile, metricFile
}

// findRecord returns the first record with the given "kind", optionally also
// matching extraKey == extraVal (when extraKey is non-empty), or nil.
func findRecord(records []map[string]any, kind, extraKey, extraVal string) map[string]any {
	for _, rec := range records {
		if rec["kind"] != kind {
			continue
		}
		if extraKey != "" && rec[extraKey] != extraVal {
			continue
		}
		return rec
	}
	return nil
}

// TestExecuteNoObservabilityEnvProducesNoFile confirms RIMBA_NO_OBSERVABILITY
// leaves zero filesystem footprint — no day-file is even created, not just
// left empty.
func TestExecuteNoObservabilityEnvProducesNoFile(t *testing.T) {
	home := redirectCacheDir(t)
	t.Setenv("RIMBA_NO_OBSERVABILITY", "1")

	dir := t.TempDir()
	cfg := &config.Config{WorktreeDir: "../worktrees"}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})
	restore := overrideNewRunner(r)
	defer restore()

	addObservabilityProbeCmd(t)

	if err := Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if files := findCacheJSONLFiles(t, home); len(files) != 0 {
		t.Errorf("expected no observability files with RIMBA_NO_OBSERVABILITY=1, found: %v", files)
	}
}

// TestExecuteConfigDisabledObservabilityProducesNoFile confirms an explicit
// [observability] enabled = false in the repo's config also produces no file.
func TestExecuteConfigDisabledObservabilityProducesNoFile(t *testing.T) {
	home := redirectCacheDir(t)

	dir := t.TempDir()
	disabled := false
	cfg := &config.Config{
		WorktreeDir:   "../worktrees",
		Observability: &config.ObservabilityConfig{Enabled: &disabled},
	}
	if err := config.Save(filepath.Join(dir, config.FileName), cfg); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	r := repoRootRunner(dir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", errors.New("unexpected")
	})
	restore := overrideNewRunner(r)
	defer restore()

	addObservabilityProbeCmd(t)

	if err := Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if files := findCacheJSONLFiles(t, home); len(files) != 0 {
		t.Errorf("expected no observability files with [observability] enabled=false, found: %v", files)
	}
}
