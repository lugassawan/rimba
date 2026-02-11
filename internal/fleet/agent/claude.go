package agent

import (
	"os/exec"

	"github.com/lugassawan/rimba/internal/fleet"
)

// Claude implements the Adapter interface for Claude Code CLI.
type Claude struct{}

func (c *Claude) Name() string { return "claude" }

func (c *Claude) Command(dir string, spec fleet.TaskSpec) *exec.Cmd {
	args := []string{"--dangerously-skip-permissions"}
	if spec.Prompt != "" {
		args = append(args, "--print", spec.Prompt)
	}
	cmd := exec.Command("claude", args...) //nolint:gosec // user-configured agent command
	cmd.Dir = dir
	return cmd
}

func (c *Claude) Available() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}
