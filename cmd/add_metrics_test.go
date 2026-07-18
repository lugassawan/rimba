package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

func TestMetricsEnabled(t *testing.T) {
	disabled := false

	tests := []struct {
		name          string
		noMetricsFlag bool
		env           string
		cfg           *config.Config
		want          bool
	}{
		{
			name:          "flag disables regardless of config",
			noMetricsFlag: true,
			cfg:           &config.Config{},
			want:          false,
		},
		{
			name: "RIMBA_METRICS=0 disables regardless of config",
			env:  "0",
			cfg:  &config.Config{},
			want: false,
		},
		{
			name: "flag and env both unset, config default (enabled) -> true",
			cfg:  &config.Config{},
			want: true,
		},
		{
			name: "flag and env both unset, config disabled -> false",
			cfg:  &config.Config{Metrics: &config.MetricsConfig{Enabled: &disabled}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("RIMBA_METRICS", tt.env)
			}
			if got := metricsEnabled(tt.cfg, tt.noMetricsFlag); got != tt.want {
				t.Errorf("metricsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAddRecordsMetrics runs a successful task-mode add with metrics enabled
// (the default) and asserts .rimba/metrics.jsonl gains exactly one line with
// the expected command and non-empty spans, proving the Recorder->Flush
// pipeline runs end-to-end.
func TestAddRecordsMetrics(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed // BranchExists returns false
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := addCmd.RunE(cmd, []string{"my-task"}); err != nil {
		t.Fatalf("addCmd.RunE: %v", err)
	}

	metricsPath := filepath.Join(repoDir, config.DirName, "metrics.jsonl")
	data, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("reading metrics.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("metrics.jsonl lines = %d, want 1 (content:\n%s)", len(lines), data)
	}

	var run struct {
		Command string `json:"command"`
		Task    string `json:"task"`
		Spans   []any  `json:"spans"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &run); err != nil {
		t.Fatalf("unmarshal run: %v", err)
	}
	if run.Command != wantAddCommand {
		t.Errorf("command = %q, want %q", run.Command, wantAddCommand)
	}
	if run.Task != "my-task" {
		t.Errorf("task = %q, want %q", run.Task, "my-task")
	}
	if len(run.Spans) == 0 {
		t.Error("spans should be non-empty (create+copy spans always run)")
	}
}

// TestAddNoMetricsFlagSkipsCollection asserts --no-metrics prevents the
// metrics file from being created at all.
func TestAddNoMetricsFlagSkipsCollection(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	cmd.Flags().Bool(flagNoMetrics, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	_ = cmd.Flags().Set(flagNoMetrics, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := addCmd.RunE(cmd, []string{"my-task"}); err != nil {
		t.Fatalf("addCmd.RunE: %v", err)
	}

	metricsPath := filepath.Join(repoDir, config.DirName, "metrics.jsonl")
	if _, err := os.Stat(metricsPath); !os.IsNotExist(err) {
		t.Fatalf("metrics.jsonl should not exist with --no-metrics, stat err = %v", err)
	}
}

// TestAddMetricsFlushFailureDoesNotFailCommand asserts a read-only .rimba/
// directory (simulating a flush failure — disk full, permissions, etc.)
// never fails the add command or changes its exit code.
func TestAddMetricsFlushFailureDoesNotFailCommand(t *testing.T) {
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	rimbaDir := filepath.Join(repoDir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(rimbaDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(rimbaDir, 0750)
	})

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := addCmd.RunE(cmd, []string{"my-task"}); err != nil {
		t.Fatalf("addCmd.RunE should not fail on metrics flush error: %v", err)
	}
	if !strings.Contains(buf.String(), "Created worktree") {
		t.Errorf("output should still report success, got:\n%s", buf.String())
	}
}

// TestAddMetricsFlushFailureLogsWhenDebugSet asserts the flush-failure debug
// line is only emitted when RIMBA_DEBUG is set, and that it still doesn't
// fail the command.
func TestAddMetricsFlushFailureLogsWhenDebugSet(t *testing.T) {
	t.Setenv("RIMBA_DEBUG", "1")

	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees")
	_ = os.MkdirAll(wtDir, 0755)
	cfg := &config.Config{DefaultSource: branchMain, WorktreeDir: "worktrees"}

	rimbaDir := filepath.Join(repoDir, config.DirName)
	if err := os.MkdirAll(rimbaDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(rimbaDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(rimbaDir, 0750)
	})

	worktreeOut := wtPrefix + repoDir + headMainBlock

	r := &mockRunner{
		run: func(args ...string) (string, error) {
			if len(args) >= 2 && args[1] == cmdGitCommonDir {
				return filepath.Join(repoDir, ".git"), nil
			}
			if len(args) >= 2 && args[1] == cmdShowToplevel {
				return repoDir, nil
			}
			if len(args) >= 1 && args[0] == cmdRevParse {
				return "", errGitFailed
			}
			return worktreeOut, nil
		},
		runInDir: noopRunInDir,
	}
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().StringP(flagSource, "s", "", "")
	cmd.Flags().Bool(flagSkipDeps, false, "")
	cmd.Flags().Bool(flagSkipHooks, false, "")
	addPrefixFlags(cmd)
	_ = cmd.Flags().Set(flagSkipDeps, "true")
	_ = cmd.Flags().Set(flagSkipHooks, "true")
	cmd.SetContext(config.WithConfig(context.Background(), cfg))

	if err := addCmd.RunE(cmd, []string{"my-task"}); err != nil {
		t.Fatalf("addCmd.RunE should not fail on metrics flush error: %v", err)
	}
	if !strings.Contains(buf.String(), "[debug] metrics flush failed") {
		t.Errorf("expected debug flush-failure log with RIMBA_DEBUG set, got:\n%s", buf.String())
	}
}
