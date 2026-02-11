package fleet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
)

// CommandFactory creates an exec.Cmd for the given agent, directory, and task.
type CommandFactory func(agentName, dir string, spec TaskSpec) *exec.Cmd

// Manager coordinates fleet task lifecycle.
type Manager struct {
	Runner     git.Runner
	Config     *config.Config
	RepoRoot   string
	StateDir   string
	LogDir     string
	NewCommand CommandFactory
}

// NewManager creates a Manager with resolved directories.
func NewManager(r git.Runner, cfg *config.Config, cmdFactory CommandFactory) (*Manager, error) {
	repoRoot, err := git.RepoRoot(r)
	if err != nil {
		return nil, err
	}

	stateDir := filepath.Join(repoRoot, cfg.FleetStateDir())
	logDir := filepath.Join(repoRoot, cfg.FleetLogDir())

	return &Manager{
		Runner:     r,
		Config:     cfg,
		RepoRoot:   repoRoot,
		StateDir:   stateDir,
		LogDir:     logDir,
		NewCommand: cmdFactory,
	}, nil
}

// EnsureDirs creates the state and log directories and adds them to .gitignore.
func (m *Manager) EnsureDirs() error {
	if err := os.MkdirAll(m.StateDir, 0750); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.MkdirAll(m.LogDir, 0750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	if _, err := fileutil.EnsureGitignore(m.RepoRoot, ".rimba/fleet/"); err != nil {
		return fmt.Errorf("update gitignore: %w", err)
	}
	return nil
}

// SpawnResult holds the result of spawning a single task.
type SpawnResult struct {
	Task  TaskSpec
	State *TaskState
	Error error
}

// Spawn creates worktrees and starts agent processes for the given tasks.
func (m *Manager) Spawn(tasks []TaskSpec, defaultAgent string) []SpawnResult {
	results := make([]SpawnResult, len(tasks))
	for i, task := range tasks {
		results[i] = m.spawnOne(task, defaultAgent)
	}
	return results
}

func (m *Manager) spawnOne(task TaskSpec, defaultAgent string) SpawnResult {
	agentName := task.Agent
	if agentName == "" {
		agentName = defaultAgent
	}

	prefix := resolvePrefix(task.Type)

	addResult, err := operations.AddWorktree(m.Runner, m.Config, task.Name, prefix, "")
	if err != nil {
		return SpawnResult{Task: task, Error: fmt.Errorf("create worktree: %w", err)}
	}

	logFile := filepath.Join(m.LogDir, task.Name+".log")
	lf, err := os.Create(logFile) //nolint:gosec // fleet log files are intentional
	if err != nil {
		return SpawnResult{Task: task, Error: fmt.Errorf("create log file: %w", err)}
	}

	cmd := m.NewCommand(agentName, addResult.Path, task)
	cmd.Stdout = lf
	cmd.Stderr = lf

	if err := cmd.Start(); err != nil {
		_ = lf.Close()
		return SpawnResult{Task: task, Error: fmt.Errorf("start agent: %w", err)}
	}

	state := &TaskState{
		Name:      task.Name,
		Agent:     agentName,
		Branch:    addResult.Branch,
		Path:      addResult.Path,
		PID:       cmd.Process.Pid,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		LogFile:   logFile,
	}

	if err := SaveState(m.StateDir, state); err != nil {
		return SpawnResult{Task: task, Error: fmt.Errorf("save state: %w", err)}
	}

	go m.monitorProcess(task.Name, cmd, lf)

	return SpawnResult{Task: task, State: state}
}

func resolvePrefix(taskType string) string {
	if taskType == "" {
		return "feature/"
	}
	if p, ok := resolver.PrefixString(resolver.PrefixType(taskType)); ok {
		return p
	}
	return "feature/"
}

func (m *Manager) monitorProcess(name string, cmd *exec.Cmd, lf *os.File) {
	_ = cmd.Wait()
	_ = lf.Close()
	st, err := LoadState(m.StateDir, name)
	if err != nil {
		return
	}
	st.Status = StatusComplete
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		st.Status = StatusFailed
	}
	_ = SaveState(m.StateDir, st)
}

// Status returns the current state of all fleet tasks with live PID checks.
func (m *Manager) Status() ([]*TaskState, error) {
	states, err := LoadAllStates(m.StateDir)
	if err != nil {
		return nil, err
	}

	for _, s := range states {
		if s.Status == StatusRunning && !IsAlive(s.PID) {
			s.Status = StatusStopped
			_ = SaveState(m.StateDir, s)
		}
	}

	return states, nil
}

// Stop terminates a running fleet task.
func (m *Manager) Stop(taskName string) error {
	state, err := LoadState(m.StateDir, taskName)
	if err != nil {
		return err
	}

	if state.Status != StatusRunning {
		return fmt.Errorf("task %q is not running (status: %s)", taskName, state.Status)
	}

	if err := StopProcess(state.PID); err != nil {
		return fmt.Errorf("stop process: %w", err)
	}

	state.Status = StatusStopped
	return SaveState(m.StateDir, state)
}

// Logs returns the path to the log file for a task.
func (m *Manager) Logs(taskName string) (string, error) {
	state, err := LoadState(m.StateDir, taskName)
	if err != nil {
		return "", err
	}
	return state.LogFile, nil
}
