package agentfile

import (
	"strings"
	"testing"
)

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
