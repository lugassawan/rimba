package agent

import (
	"os/exec"

	"github.com/lugassawan/rimba/internal/fleet"
)

// Codex implements the Adapter interface for OpenAI Codex.
type Codex struct{}

func (c *Codex) Name() string { return "codex" }

func (c *Codex) Command(dir string, spec fleet.TaskSpec) *exec.Cmd {
	args := []string{}
	if spec.Prompt != "" {
		args = append(args, spec.Prompt)
	}
	cmd := exec.Command("codex", args...) //nolint:gosec // user-configured agent command
	cmd.Dir = dir
	return cmd
}

func (c *Codex) Available() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}
