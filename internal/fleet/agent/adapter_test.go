package agent

import (
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/fleet"
)

const (
	agentClaude      = "claude"
	agentCursor      = "cursor"
	agentCodex       = "codex"
	workDir          = "/work"
	testTaskName     = "task-a"
	customAgentName  = "my-custom-agent"
	genericAgentName = "my-agent"
	promptAddFeature = "add feature"
	fmtNameWant      = "Name() = %q, want %q"
	fmtDirWant       = "Dir = %q, want %q"
)

func TestResolveKnownAgents(t *testing.T) {
	tests := []struct {
		name     string
		wantType string
	}{
		{"claude", "*agent.Claude"},
		{"cursor", "*agent.Cursor"},
		{"codex", "*agent.Codex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Resolve(tt.name)
			if a.Name() != tt.name {
				t.Errorf(fmtNameWant, a.Name(), tt.name)
			}
		})
	}
}

func TestResolveUnknownReturnsGeneric(t *testing.T) {
	a := Resolve(customAgentName)
	if a.Name() != customAgentName {
		t.Errorf(fmtNameWant, a.Name(), customAgentName)
	}
	g, ok := a.(*Generic)
	if !ok {
		t.Fatalf("expected *Generic, got %T", a)
	}
	if g.AgentName != customAgentName {
		t.Errorf("AgentName = %q, want %q", g.AgentName, customAgentName)
	}
}

func TestClaudeAdapter(t *testing.T) {
	c := &Claude{}

	if c.Name() != agentClaude {
		t.Errorf(fmtNameWant, c.Name(), agentClaude)
	}

	spec := fleet.TaskSpec{Name: testTaskName, Prompt: "fix the bug"}
	cmd := c.Command(workDir, spec)

	if cmd.Dir != workDir {
		t.Errorf(fmtDirWant, cmd.Dir, workDir)
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "--dangerously-skip-permissions") {
		t.Errorf("expected --dangerously-skip-permissions in args: %s", args)
	}
	if !strings.Contains(args, "--print") {
		t.Errorf("expected --print in args: %s", args)
	}
}

func TestClaudeNoPrompt(t *testing.T) {
	c := &Claude{}
	spec := fleet.TaskSpec{Name: testTaskName}
	cmd := c.Command(workDir, spec)

	args := strings.Join(cmd.Args, " ")
	if strings.Contains(args, "--print") {
		t.Errorf("--print should not be present without prompt: %s", args)
	}
}

func TestCursorAdapter(t *testing.T) {
	c := &Cursor{}

	if c.Name() != agentCursor {
		t.Errorf(fmtNameWant, c.Name(), agentCursor)
	}

	spec := fleet.TaskSpec{Name: testTaskName, Prompt: promptAddFeature}
	cmd := c.Command(workDir, spec)

	if cmd.Dir != workDir {
		t.Errorf(fmtDirWant, cmd.Dir, workDir)
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "--background") {
		t.Errorf("expected --background in args: %s", args)
	}
}

func TestCursorNoPrompt(t *testing.T) {
	c := &Cursor{}
	spec := fleet.TaskSpec{Name: testTaskName}
	cmd := c.Command(workDir, spec)
	// Should have only --background.
	if len(cmd.Args) != 2 { // cursor --background
		t.Errorf("expected 2 args without prompt, got %d: %v", len(cmd.Args), cmd.Args)
	}
}

func TestCodexAdapter(t *testing.T) {
	c := &Codex{}

	if c.Name() != agentCodex {
		t.Errorf(fmtNameWant, c.Name(), agentCodex)
	}

	spec := fleet.TaskSpec{Name: testTaskName, Prompt: promptAddFeature}
	cmd := c.Command(workDir, spec)

	if cmd.Dir != workDir {
		t.Errorf(fmtDirWant, cmd.Dir, workDir)
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, promptAddFeature) {
		t.Errorf("expected prompt in args: %s", args)
	}
}

func TestCodexNoPrompt(t *testing.T) {
	c := &Codex{}
	spec := fleet.TaskSpec{Name: testTaskName}
	cmd := c.Command(workDir, spec)
	// Should have only "codex" with no extra args.
	if len(cmd.Args) != 1 {
		t.Errorf("expected 1 arg without prompt, got %d: %v", len(cmd.Args), cmd.Args)
	}
}

func TestGenericAdapter(t *testing.T) {
	g := &Generic{AgentName: genericAgentName}

	if g.Name() != genericAgentName {
		t.Errorf(fmtNameWant, g.Name(), genericAgentName)
	}

	spec := fleet.TaskSpec{Name: testTaskName, Prompt: "echo hello"}
	cmd := g.Command(workDir, spec)

	if cmd.Dir != workDir {
		t.Errorf(fmtDirWant, cmd.Dir, workDir)
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "-c") {
		t.Errorf("expected -c flag in args: %s", args)
	}
	if !strings.Contains(args, "echo hello") {
		t.Errorf("expected prompt in args: %s", args)
	}
}

func TestGenericNoPrompt(t *testing.T) {
	g := &Generic{AgentName: genericAgentName}
	spec := fleet.TaskSpec{Name: testTaskName}
	cmd := g.Command(workDir, spec)
	// Without prompt, no -c flag should be present.
	args := strings.Join(cmd.Args, " ")
	if strings.Contains(args, "-c") {
		t.Errorf("-c flag should not be present without prompt: %s", args)
	}
}

func TestGenericAvailable(t *testing.T) {
	g := &Generic{}
	// sh should be available on all Unix systems.
	if !g.Available() {
		t.Error("Generic.Available() should be true (sh exists)")
	}
}

func TestClaudeAvailable(t *testing.T) {
	c := &Claude{}
	// Just exercise Available() — result depends on whether claude is installed.
	_ = c.Available()
}

func TestCursorAvailable(t *testing.T) {
	c := &Cursor{}
	_ = c.Available()
}

func TestCodexAvailable(t *testing.T) {
	c := &Codex{}
	_ = c.Available()
}
