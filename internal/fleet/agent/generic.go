package agent

import (
	"os/exec"

	"github.com/lugassawan/rimba/internal/fleet"
)

// Generic implements the Adapter interface for arbitrary commands.
type Generic struct {
	AgentName string
}

func (g *Generic) Name() string { return g.AgentName }

func (g *Generic) Command(dir string, spec fleet.TaskSpec) *exec.Cmd {
	args := []string{}
	if spec.Prompt != "" {
		args = append(args, "-c", spec.Prompt)
	}
	cmd := exec.Command("sh", args...) //nolint:gosec // user-configured command
	cmd.Dir = dir
	return cmd
}

func (g *Generic) Available() bool {
	_, err := exec.LookPath("sh")
	return err == nil
}
