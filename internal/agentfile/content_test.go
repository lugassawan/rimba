package agentfile

import (
	"strings"
	"testing"
)

type labeledSpec struct {
	label string
	spec  Spec
}

func TestAgentsBlockHasMarkers(t *testing.T) {
	content := agentsBlock()
	if !strings.HasPrefix(content, BeginMarker) {
		t.Error("agents block should start with BEGIN marker")
	}
	if !strings.HasSuffix(content, EndMarker) {
		t.Error("agents block should end with END marker")
	}
}

func TestCopilotBlockHasMarkers(t *testing.T) {
	content := copilotBlock()
	if !strings.HasPrefix(content, BeginMarker) {
		t.Error("copilot block should start with BEGIN marker")
	}
	if !strings.HasSuffix(content, EndMarker) {
		t.Error("copilot block should end with END marker")
	}
}

func TestCursorContentHasFrontmatter(t *testing.T) {
	content := cursorContent()
	if !strings.HasPrefix(content, "---\n") {
		t.Error("cursor content should start with YAML frontmatter")
	}
	if !strings.Contains(content, "description:") {
		t.Error("cursor content should have description field")
	}
	if !strings.Contains(content, "globs:") {
		t.Error("cursor content should have globs field")
	}
}

func TestClaudeSkillContentHasFrontmatter(t *testing.T) {
	content := claudeSkillContent()
	if !strings.HasPrefix(content, "---\n") {
		t.Error("claude skill content should start with YAML frontmatter")
	}
	if !strings.Contains(content, "name: rimba") {
		t.Error("claude skill content should have name field")
	}
	if !strings.Contains(content, "description:") {
		t.Error("claude skill content should have description field")
	}
}

func TestGeminiBlockHasMarkers(t *testing.T) {
	content := geminiBlock()
	if !strings.HasPrefix(content, BeginMarker) {
		t.Error("gemini block should start with BEGIN marker")
	}
	if !strings.HasSuffix(content, EndMarker) {
		t.Error("gemini block should end with END marker")
	}
	if !strings.Contains(content, "rimba") {
		t.Error("gemini block should mention rimba")
	}
}

func TestWindsurfContentNotEmpty(t *testing.T) {
	content := windsurfContent()
	if content == "" {
		t.Error("windsurf content should not be empty")
	}
	if !strings.Contains(content, "rimba") {
		t.Error("windsurf content should mention rimba")
	}
}

func TestRooContentNotEmpty(t *testing.T) {
	content := rooContent()
	if content == "" {
		t.Error("roo content should not be empty")
	}
	if !strings.Contains(content, "rimba") {
		t.Error("roo content should mention rimba")
	}
}

func TestGlobalClaudeSkillContentHasFrontmatter(t *testing.T) {
	content := globalClaudeSkillContent()
	if !strings.HasPrefix(content, "---\n") {
		t.Error("global claude skill content should start with YAML frontmatter")
	}
	if !strings.Contains(content, "name: rimba") {
		t.Error("global claude skill content should have name field")
	}
	if !strings.Contains(content, "description:") {
		t.Error("global claude skill content should have description field")
	}
}

func TestGlobalCursorContentHasFrontmatterNoGlobs(t *testing.T) {
	content := globalCursorContent()
	if !strings.HasPrefix(content, "---\n") {
		t.Error("global cursor content should start with YAML frontmatter")
	}
	if !strings.Contains(content, "alwaysApply: true") {
		t.Error("global cursor content should have alwaysApply: true")
	}
	if strings.Contains(content, "globs:") {
		t.Error("global cursor content should not have globs field")
	}
}

func TestGlobalCopilotBlockHasMarkers(t *testing.T) {
	content := globalCopilotBlock()
	if !strings.HasPrefix(content, BeginMarker) {
		t.Error("global copilot block should start with BEGIN marker")
	}
	if !strings.HasSuffix(content, EndMarker) {
		t.Error("global copilot block should end with END marker")
	}
}

func TestGlobalCodexBlockHasMarkers(t *testing.T) {
	content := globalCodexBlock()
	if !strings.HasPrefix(content, BeginMarker) {
		t.Error("global codex block should start with BEGIN marker")
	}
	if !strings.HasSuffix(content, EndMarker) {
		t.Error("global codex block should end with END marker")
	}
}

func TestGlobalGeminiBlockHasMarkers(t *testing.T) {
	content := globalGeminiBlock()
	if !strings.HasPrefix(content, BeginMarker) {
		t.Error("global gemini block should start with BEGIN marker")
	}
	if !strings.HasSuffix(content, EndMarker) {
		t.Error("global gemini block should end with END marker")
	}
}

func TestGlobalWindsurfBlockHasMarkers(t *testing.T) {
	content := globalWindsurfBlock()
	if !strings.HasPrefix(content, BeginMarker) {
		t.Error("global windsurf block should start with BEGIN marker")
	}
	if !strings.HasSuffix(content, EndMarker) {
		t.Error("global windsurf block should end with END marker")
	}
}

func TestGlobalRooContentNotEmpty(t *testing.T) {
	content := globalRooContent()
	if content == "" {
		t.Error("global roo content should not be empty")
	}
	if !strings.Contains(content, "rimba") {
		t.Error("global roo content should mention rimba")
	}
}

func TestMcpToolsSection(t *testing.T) {
	cases := []struct {
		name    string
		heading string
	}{
		{"h2", "##"},
		{"h3", "###"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			section := mcpToolsSection(c.heading)

			if !strings.HasPrefix(section, c.heading+" MCP Tools") {
				t.Errorf("mcp tools section should start with %q heading", c.heading+" MCP Tools")
			}
			if !strings.Contains(section, "prefer the native") || !strings.Contains(section, "mcp__rimba__*") {
				t.Error("mcp tools section should explain preferring native mcp__rimba__* tools")
			}
			if !strings.Contains(section, "Fall back to the CLI") {
				t.Error("mcp tools section should explain the CLI fallback")
			}

			for _, tool := range mcpToolEntries {
				if !strings.Contains(section, tool.mcp) {
					t.Errorf("mcp tools section should mention %s", tool.mcp)
				}
				if !strings.Contains(section, tool.cli) {
					t.Errorf("mcp tools section should mention CLI equivalent %s", tool.cli)
				}
			}
		})
	}
}

func TestMcpToolEntriesIncludesAllRegisteredTools(t *testing.T) {
	want := []string{
		"mcp__rimba__add",
		"mcp__rimba__list",
		"mcp__rimba__status",
		"mcp__rimba__sync",
		"mcp__rimba__merge",
		"mcp__rimba__remove",
		"mcp__rimba__clean",
		"mcp__rimba__exec",
		"mcp__rimba__conflict-check",
		"mcp__rimba__rename",
		"mcp__rimba__merge-plan",
		"mcp__rimba__log",
		"mcp__rimba__archive",
		"mcp__rimba__restore",
	}

	got := make(map[string]bool, len(mcpToolEntries))
	for _, e := range mcpToolEntries {
		got[e.mcp] = true
	}

	if len(mcpToolEntries) != len(want) {
		t.Errorf("mcpToolEntries has %d entries, want %d", len(mcpToolEntries), len(want))
	}
	for _, mcp := range want {
		if !got[mcp] {
			t.Errorf("mcpToolEntries missing %s", mcp)
		}
	}
}

func TestProjectGeneratorsMentionCurrentCommands(t *testing.T) {
	cases := []struct {
		name    string
		content func() string
	}{
		{"agentsBlock", agentsBlock},
		{"copilotBlock", copilotBlock},
		{"cursorContent", cursorContent},
		{"geminiBlock", geminiBlock},
		{"windsurfContent", windsurfContent},
		{"rooContent", rooContent},
		{"claudeSkillContent", claudeSkillContent},
	}

	commands := []string{"doctor", "rename", "restore", "duplicate", "trust"}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content := c.content()
			for _, cmd := range commands {
				if !strings.Contains(content, cmd) {
					t.Errorf("%s should mention %q command", c.name, cmd)
				}
			}
		})
	}
}

func TestProjectJSONCommandListsAreCurrent(t *testing.T) {
	wantCommands := []string{
		"list", "status", "exec", "conflict-check", "deps status",
		"add", "merge", "remove", "rename", "sync", "clean", "log",
	}

	cases := []struct {
		name    string
		content func() string
	}{
		{"agentsBlock", agentsBlock},
		{"cursorContent", cursorContent},
		{"claudeSkillContent", claudeSkillContent},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content := c.content()
			for _, cmd := range wantCommands {
				if !strings.Contains(content, cmd) {
					t.Errorf("%s --json list should mention %q", c.name, cmd)
				}
			}
		})
	}
}

func TestAllSpecsIncludeMcpToolsSection(t *testing.T) {
	projectSpecs := ProjectSpecs()
	globalSpecs := GlobalSpecs()

	specs := make([]labeledSpec, 0, len(projectSpecs)+len(globalSpecs))
	for _, s := range projectSpecs {
		specs = append(specs, labeledSpec{"project", s})
	}
	for _, s := range globalSpecs {
		specs = append(specs, labeledSpec{"global", s})
	}

	for _, ls := range specs {
		content := ls.spec.Content()
		for _, tool := range mcpToolEntries {
			if !strings.Contains(content, tool.mcp) {
				t.Errorf("%s spec %s should mention %s", ls.label, ls.spec.RelPath, tool.mcp)
			}
		}
	}
}
