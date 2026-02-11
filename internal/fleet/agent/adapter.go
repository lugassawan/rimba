package agent

import (
	"os/exec"

	"github.com/lugassawan/rimba/internal/fleet"
)

// Adapter knows how to launch a specific AI tool.
type Adapter interface {
	Name() string
	Command(dir string, spec fleet.TaskSpec) *exec.Cmd
	Available() bool
}

// Resolve returns the adapter for the given agent name.
func Resolve(name string) Adapter {
	switch name {
	case "claude":
		return &Claude{}
	case "cursor":
		return &Cursor{}
	case "codex":
		return &Codex{}
	default:
		return &Generic{AgentName: name}
	}
}
