package fleet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lugassawan/rimba/internal/config"
)

const (
	testRepoRoot      = "/repo"
	testFeaturePrefix = "feature/"
	testStateDir      = ".rimba/fleet"
	testLogDir        = ".rimba/fleet/logs"
	testLogFilePath   = "/logs/my-task.log"
	fmtStatusErr      = "Status: %v"
)

func TestResolvePrefix(t *testing.T) {
	tests := []struct {
		name     string
		taskType string
		want     string
	}{
		{"empty defaults to feature", "", testFeaturePrefix},
		{"feature type", "feature", testFeaturePrefix},
		{"bugfix type", "bugfix", "bugfix/"},
		{"hotfix type", "hotfix", "hotfix/"},
		{"docs type", "docs", "docs/"},
		{"test type", "test", "test/"},
		{"chore type", "chore", "chore/"},
		{"unknown defaults to feature", "unknown-type", testFeaturePrefix},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePrefix(tt.taskType)
			if got != tt.want {
				t.Errorf("resolvePrefix(%q) = %q, want %q", tt.taskType, got, tt.want)
			}
		})
	}
}

// mockRunner implements git.Runner for testing.
type mockRunner struct {
	runFunc      func(args ...string) (string, error)
	runInDirFunc func(dir string, args ...string) (string, error)
}

func (m *mockRunner) Run(args ...string) (string, error) {
	return m.runFunc(args...)
}

func (m *mockRunner) RunInDir(dir string, args ...string) (string, error) {
	return m.runInDirFunc(dir, args...)
}

func TestNewManager(t *testing.T) {
	r := &mockRunner{
		runFunc: func(_ ...string) (string, error) {
			return testRepoRoot, nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   "../worktrees",
		DefaultSource: "main",
		Fleet: &config.FleetConfig{
			StateDir: testStateDir,
			LogDir:   testLogDir,
		},
	}
	mgr, err := NewManager(r, cfg, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if mgr.RepoRoot != testRepoRoot {
		t.Errorf("RepoRoot = %q, want %q", mgr.RepoRoot, testRepoRoot)
	}
	if mgr.StateDir != filepath.Join(testRepoRoot, testStateDir) {
		t.Errorf("StateDir = %q, want %q", mgr.StateDir, filepath.Join(testRepoRoot, testStateDir))
	}
	if mgr.LogDir != filepath.Join(testRepoRoot, testLogDir) {
		t.Errorf("LogDir = %q, want %q", mgr.LogDir, filepath.Join(testRepoRoot, testLogDir))
	}
}

func TestNewManagerDefaultDirs(t *testing.T) {
	r := &mockRunner{
		runFunc: func(_ ...string) (string, error) {
			return testRepoRoot, nil
		},
	}

	cfg := &config.Config{
		WorktreeDir:   "../worktrees",
		DefaultSource: "main",
	}
	mgr, err := NewManager(r, cfg, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Should use defaults when Fleet config is nil.
	if mgr.StateDir != filepath.Join(testRepoRoot, testStateDir) {
		t.Errorf("StateDir = %q, want default", mgr.StateDir)
	}
	if mgr.LogDir != filepath.Join(testRepoRoot, testLogDir) {
		t.Errorf("LogDir = %q, want default", mgr.LogDir)
	}
}

func TestManagerStatus(t *testing.T) {
	dir := t.TempDir()

	states := []*TaskState{
		{Name: "running-task", PID: os.Getpid(), Status: StatusRunning},
		{Name: "done-task", PID: 99999999, Status: StatusComplete},
	}
	for _, s := range states {
		if err := SaveState(dir, s); err != nil {
			t.Fatal(err)
		}
	}

	mgr := &Manager{StateDir: dir}
	result, err := mgr.Status()
	if err != nil {
		t.Fatalf(fmtStatusErr, err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d states, want 2", len(result))
	}
}

func TestManagerStatusEmpty(t *testing.T) {
	dir := t.TempDir()
	mgr := &Manager{StateDir: dir}
	result, err := mgr.Status()
	if err != nil {
		t.Fatalf(fmtStatusErr, err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 states, got %d", len(result))
	}
}

func TestManagerStatusNonexistentDir(t *testing.T) {
	mgr := &Manager{StateDir: "/nonexistent/dir"}
	result, err := mgr.Status()
	if err != nil {
		t.Fatalf("Status should handle nonexistent dir: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil states, got %v", result)
	}
}

func TestManagerStop(t *testing.T) {
	dir := t.TempDir()
	state := &TaskState{Name: "stopped-task", Status: StatusStopped}
	if err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	mgr := &Manager{StateDir: dir}
	err := mgr.Stop("stopped-task")
	if err == nil {
		t.Fatal("expected error when stopping non-running task")
	}
}

func TestManagerStopNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := &Manager{StateDir: dir}
	err := mgr.Stop("nonexistent")
	if err == nil {
		t.Fatal("expected error when stopping nonexistent task")
	}
}

func TestManagerLogs(t *testing.T) {
	dir := t.TempDir()
	state := &TaskState{Name: "my-task", LogFile: testLogFilePath}
	if err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	mgr := &Manager{StateDir: dir}
	logPath, err := mgr.Logs("my-task")
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if logPath != testLogFilePath {
		t.Errorf("LogFile = %q, want %q", logPath, testLogFilePath)
	}
}

func TestManagerLogsNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := &Manager{StateDir: dir}
	_, err := mgr.Logs("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestManagerEnsureDirs(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")
	logDir := filepath.Join(tmpDir, "logs")

	mgr := &Manager{
		RepoRoot: tmpDir,
		StateDir: stateDir,
		LogDir:   logDir,
	}

	if err := mgr.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	if _, err := os.Stat(stateDir); err != nil {
		t.Errorf("state dir should exist: %v", err)
	}
	if _, err := os.Stat(logDir); err != nil {
		t.Errorf("log dir should exist: %v", err)
	}
}

func TestManagerStopRunningButDeadProcess(t *testing.T) {
	dir := t.TempDir()
	// Save a state with "running" status but a dead PID.
	state := &TaskState{Name: "dead-task", PID: 99999999, Status: StatusRunning}
	if err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	mgr := &Manager{StateDir: dir}
	err := mgr.Stop("dead-task")
	// Should attempt to stop but the signal will fail since the process doesn't exist.
	if err == nil {
		t.Fatal("expected error when stopping dead process")
	}
}

func TestManagerSpawnEmpty(t *testing.T) {
	mgr := &Manager{}
	results := mgr.Spawn(nil, "claude")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestManagerStatusReconcilesStalePIDs(t *testing.T) {
	dir := t.TempDir()

	state := &TaskState{Name: "stale-task", PID: 99999999, Status: StatusRunning}
	if err := SaveState(dir, state); err != nil {
		t.Fatal(err)
	}

	mgr := &Manager{StateDir: dir}
	result, err := mgr.Status()
	if err != nil {
		t.Fatalf(fmtStatusErr, err)
	}
	if len(result) != 1 {
		t.Fatalf("got %d states, want 1", len(result))
	}
	if result[0].Status != StatusStopped {
		t.Errorf("stale task status = %q, want %q", result[0].Status, StatusStopped)
	}
}
