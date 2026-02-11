package agent

import (
	"os/exec"

	"github.com/lugassawan/rimba/internal/fleet"
)

// Cursor implements the Adapter interface for Cursor background agent.
type Cursor struct{}

func (c *Cursor) Name() string { return "cursor" }

func (c *Cursor) Command(dir string, spec fleet.TaskSpec) *exec.Cmd {
	args := []string{"--background"}
	if spec.Prompt != "" {
		args = append(args, spec.Prompt)
	}
	cmd := exec.Command("cursor", args...) //nolint:gosec // user-configured agent command
	cmd.Dir = dir
	return cmd
}

func (c *Cursor) Available() bool {
	_, err := exec.LookPath("cursor")
	return err == nil
}
