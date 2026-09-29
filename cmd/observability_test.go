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
	"github.com/lugassawan/rimba/internal/output"
	"github.com/spf13/cobra"
)

var errProbeBoom = errors.New("probe boom")

// addObservabilityProbeCmd registers a throwaway subcommand (deliberately not
// skipConfig-annotated, unlike version/status) so PersistentPreRunE runs its
// full path — including the observability build — when Execute() invokes it.
// Returns a cleanup func that removes it and resets rootCmd's execution state
// (mirroring TestExecute's cleanup). A nil runE makes the probe a no-op.
func addObservabilityProbeCmd(t *testing.T, runE func(*cobra.Command, []string) error) {
	t.Helper()
	if runE == nil {
		runE = func(*cobra.Command, []string) error { return nil }
	}
	probe := &cobra.Command{
		Use:  "observability-probe",
		RunE: runE,
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

	addObservabilityProbeCmd(t, nil)

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

	addObservabilityProbeCmd(t, nil)

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

	addObservabilityProbeCmd(t, nil)

	if err := Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if files := findCacheJSONLFiles(t, home); len(files) != 0 {
		t.Errorf("expected no observability files with [observability] enabled=false, found: %v", files)
	}
}

// executeRecoveringPanic runs Execute() and returns the value it panicked
// with (nil if it returned normally).
func executeRecoveringPanic(t *testing.T) (recovered any) {
	t.Helper()
	defer func() { recovered = recover() }()
	_ = Execute()
	return nil
}

func TestExecutePanicRecordsErrorAndRepanics(t *testing.T) {
	home := redirectCacheDir(t)

	dir := t.TempDir()
	if err := config.Save(filepath.Join(dir, config.FileName), &config.Config{WorktreeDir: "../worktrees"}); err != nil {
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

	addObservabilityProbeCmd(t, func(*cobra.Command, []string) error { panic(errProbeBoom) })

	if got := executeRecoveringPanic(t); got != errProbeBoom { //nolint:errorlint // asserting identity of the re-panicked value
		t.Fatalf("Execute panic value = %v, want errProbeBoom", got)
	}

	logFile, metricFile := splitDayFiles(t, findCacheJSONLFiles(t, home))
	cmdRecord := findRecord(jsonlRecords(t, logFile), "command", "", "")
	if cmdRecord == nil {
		t.Fatalf("expected a CommandRecord in %s", logFile)
	}
	if cmdRecord["outcome"] != "error" {
		t.Errorf("outcome = %v, want error", cmdRecord["outcome"])
	}
	if code, _ := cmdRecord["exit_code"].(float64); code != 2 {
		t.Errorf("exit_code = %v, want 2", cmdRecord["exit_code"])
	}
	if msg, _ := cmdRecord["error"].(string); !strings.Contains(msg, "panic:") {
		t.Errorf("error = %q, want it to contain %q", msg, "panic:")
	}
	rootSpan := findRecord(jsonlRecords(t, metricFile), "span", "name", "command")
	if rootSpan == nil {
		t.Fatalf("expected a root SpanRecord in %s", metricFile)
	}
	if rootSpan["run_id"] != cmdRecord["run_id"] {
		t.Errorf("run_id mismatch: span %v vs command %v", rootSpan["run_id"], cmdRecord["run_id"])
	}
}

func TestFinalizeRecorderNilRecorderStillRepanics(t *testing.T) {
	defer func() {
		if got := recover(); got != "x" {
			t.Errorf("recovered %v, want x", got)
		}
	}()
	finalizeRecorder(nil, "x", nil)
	t.Fatal("expected finalizeRecorder to re-panic")
}

func TestFinalizeRecorderNilRecorderErrorNoPanic(t *testing.T) {
	finalizeRecorder(nil, nil, errors.New("boom"))
}

func TestExecuteErrorRecordsOutcomeAndExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode float64
	}{
		{"plain error", errors.New("probe failure"), 1},
		{"silent error", &output.SilentError{ExitCode: 7}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := redirectCacheDir(t)
			dir := t.TempDir()
			if err := config.Save(filepath.Join(dir, config.FileName), &config.Config{WorktreeDir: "../worktrees"}); err != nil {
				t.Fatalf("Save config: %v", err)
			}
			restore := overrideNewRunner(repoRootRunner(dir, func(args ...string) (string, error) {
				if args[0] == cmdSymbolicRef {
					return refsRemotesOriginMain, nil
				}
				return "", errors.New("unexpected")
			}))
			defer restore()

			addObservabilityProbeCmd(t, func(*cobra.Command, []string) error { return tt.err })
			if err := Execute(); err == nil {
				t.Fatal("expected Execute to return the probe error")
			}

			logFile, _ := splitDayFiles(t, findCacheJSONLFiles(t, home))
			rec := findRecord(jsonlRecords(t, logFile), "command", "", "")
			if rec == nil {
				t.Fatalf("expected a CommandRecord in %s", logFile)
			}
			if rec["outcome"] != "error" {
				t.Errorf("outcome = %v, want error", rec["outcome"])
			}
			if code, _ := rec["exit_code"].(float64); code != tt.wantCode {
				t.Errorf("exit_code = %v, want %v", rec["exit_code"], tt.wantCode)
			}
		})
	}
}

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"plain", errors.New("boom"), 1},
		{"silent", &output.SilentError{ExitCode: 7}, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeFor(tt.err); got != tt.want {
				t.Errorf("exitCodeFor = %d, want %d", got, tt.want)
			}
		})
	}
}
