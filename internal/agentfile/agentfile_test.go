package agentfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	fatalInstall   = "Install: %v"
	fatalUninstall = "Uninstall: %v"
	fatalRead      = "read file: %v"
)

// --- Specs tests ---

func TestSpecsReturnsFourFiles(t *testing.T) {
	specs := Specs()
	if len(specs) != 4 {
		t.Fatalf("Specs() returned %d items, want 4", len(specs))
	}
}

func TestSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range Specs() {
		content := spec.Content()
		if content == "" {
			t.Errorf("Spec %q returned empty content", spec.RelPath)
		}
	}
}

// --- Install tests ---

func TestInstallCreatesAllFiles(t *testing.T) {
	dir := t.TempDir()

	results, err := Install(dir)
	if err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if len(results) != 4 {
		t.Fatalf("Install returned %d results, want 4", len(results))
	}

	for _, r := range results {
		if r.Action != actionCreated {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionCreated)
		}
		path := filepath.Join(dir, r.RelPath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("%s: file not created", r.RelPath)
		}
	}
}

func TestInstallBlockBased(t *testing.T) {
	dir := t.TempDir()

	if _, err := Install(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	// Check AGENTS.md has markers
	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf(fatalRead, err)
	}

	s := string(content)
	if !strings.Contains(s, BeginMarker) {
		t.Error("AGENTS.md missing BEGIN marker")
	}
	if !strings.Contains(s, EndMarker) {
		t.Error("AGENTS.md missing END marker")
	}
	if !strings.Contains(s, "rimba") {
		t.Error("AGENTS.md missing rimba content")
	}
}

func TestInstallAppendToExisting(t *testing.T) {
	dir := t.TempDir()

	// Create existing AGENTS.md with user content
	existing := "# My Project\n\nSome user documentation.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(existing), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	results, err := Install(dir)
	if err != nil {
		t.Fatalf(fatalInstall, err)
	}

	// AGENTS.md should be "updated" not "created"
	for _, r := range results {
		if r.RelPath == "AGENTS.md" && r.Action != actionUpdated {
			t.Errorf("AGENTS.md action = %q, want %q", r.Action, actionUpdated)
		}
	}

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf(fatalRead, err)
	}

	s := string(content)
	if !strings.Contains(s, "My Project") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(s, BeginMarker) {
		t.Error("rimba block should be appended")
	}
}

func TestInstallAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()

	if _, err := Install(dir); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	// Second install should succeed (idempotent)
	results, err := Install(dir)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}

	for _, r := range results {
		if r.Action != actionUpdated {
			t.Errorf("%s: action = %q, want %q on re-install", r.RelPath, r.Action, actionUpdated)
		}
	}

	// Verify no duplicated markers in AGENTS.md
	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf(fatalRead, err)
	}

	s := string(content)
	if strings.Count(s, BeginMarker) != 1 {
		t.Errorf("AGENTS.md has %d BEGIN markers, want 1", strings.Count(s, BeginMarker))
	}
	if strings.Count(s, EndMarker) != 1 {
		t.Errorf("AGENTS.md has %d END markers, want 1", strings.Count(s, EndMarker))
	}
}

func TestInstallCreatesDirectories(t *testing.T) {
	dir := t.TempDir()

	if _, err := Install(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	// .cursor/rules/ should exist
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules")); os.IsNotExist(err) {
		t.Error(".cursor/rules/ directory not created")
	}

	// .claude/skills/rimba/ should exist
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "rimba")); os.IsNotExist(err) {
		t.Error(".claude/skills/rimba/ directory not created")
	}
}

func TestInstallPreservesExistingContentWithBlock(t *testing.T) {
	dir := t.TempDir()

	// Existing file with user content and a rimba block
	existing := "# My Agents\n\n" + BeginMarker + "\nold rimba content\n" + EndMarker + "\n\n# Other Section\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(existing), 0644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	if _, err := Install(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf(fatalRead, err)
	}

	s := string(content)
	if !strings.Contains(s, "My Agents") {
		t.Error("content before block should be preserved")
	}
	if !strings.Contains(s, "Other Section") {
		t.Error("content after block should be preserved")
	}
	if strings.Contains(s, "old rimba content") {
		t.Error("old block content should be replaced")
	}
	if strings.Count(s, BeginMarker) != 1 {
		t.Errorf("should have exactly 1 BEGIN marker, got %d", strings.Count(s, BeginMarker))
	}
}

// --- Uninstall tests ---

func TestUninstallRemovesBlock(t *testing.T) {
	dir := t.TempDir()

	// Create AGENTS.md with user content + rimba block
	userContent := "# My Agents\n"
	content := userContent + "\n" + agentsBlock() + "\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Also install whole-file types
	if _, err := Install(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	results, err := Uninstall(dir)
	if err != nil {
		t.Fatalf(fatalUninstall, err)
	}

	if len(results) != 4 {
		t.Fatalf("Uninstall returned %d results, want 4", len(results))
	}

	// AGENTS.md should still exist with user content
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf(fatalRead, err)
	}
	s := string(data)
	if strings.Contains(s, BeginMarker) {
		t.Error("AGENTS.md should not contain BEGIN marker after uninstall")
	}
	if !strings.Contains(s, "My Agents") {
		t.Error("user content should be preserved after uninstall")
	}
}

func TestUninstallRemovesWholeFile(t *testing.T) {
	dir := t.TempDir()

	if _, err := Install(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if _, err := Uninstall(dir); err != nil {
		t.Fatalf(fatalUninstall, err)
	}

	// Whole-file types should be deleted
	cursorPath := filepath.Join(dir, ".cursor", "rules", "rimba.mdc")
	if _, err := os.Stat(cursorPath); !os.IsNotExist(err) {
		t.Error(".cursor/rules/rimba.mdc should be deleted after uninstall")
	}

	claudePath := filepath.Join(dir, ".claude", "skills", "rimba", "SKILL.md")
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Error(".claude/skills/rimba/SKILL.md should be deleted after uninstall")
	}
}

func TestUninstallRemovesEmptyBlockFile(t *testing.T) {
	dir := t.TempDir()

	// Install creates AGENTS.md with only rimba content
	if _, err := Install(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if _, err := Uninstall(dir); err != nil {
		t.Fatalf(fatalUninstall, err)
	}

	// AGENTS.md should be deleted (no user content)
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Error("AGENTS.md should be deleted when only rimba content was present")
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	dir := t.TempDir()

	results, err := Uninstall(dir)
	if err != nil {
		t.Fatalf(fatalUninstall, err)
	}

	for _, r := range results {
		if r.Action != actionSkipped {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionSkipped)
		}
	}
}

// --- Status tests ---

func TestStatusNotInstalled(t *testing.T) {
	dir := t.TempDir()

	statuses := Status(dir)
	if len(statuses) != 4 {
		t.Fatalf("Status returned %d items, want 4", len(statuses))
	}

	for _, s := range statuses {
		if s.Installed {
			t.Errorf("%s: expected Installed = false", s.RelPath)
		}
	}
}

func TestStatusInstalled(t *testing.T) {
	dir := t.TempDir()

	if _, err := Install(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	statuses := Status(dir)
	for _, s := range statuses {
		if !s.Installed {
			t.Errorf("%s: expected Installed = true", s.RelPath)
		}
	}
}

func TestStatusPartialInstall(t *testing.T) {
	dir := t.TempDir()

	// Only create AGENTS.md with the block
	content := agentsBlock() + "\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	statuses := Status(dir)
	foundAgents := false
	for _, s := range statuses {
		if s.RelPath == "AGENTS.md" {
			foundAgents = true
			if !s.Installed {
				t.Error("AGENTS.md should be Installed = true")
			}
		} else if s.Installed {
			t.Errorf("%s: expected Installed = false (not created)", s.RelPath)
		}
	}
	if !foundAgents {
		t.Error("AGENTS.md not found in status results")
	}
}

// --- Block manipulation tests ---

func TestContainsBlock(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"both markers", BeginMarker + "\ncontent\n" + EndMarker, true},
		{"begin only", BeginMarker + "\ncontent", false},
		{"end only", "content\n" + EndMarker, false},
		{"no markers", "just content", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsBlock(tt.content); got != tt.want {
				t.Errorf("containsBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveBlock(t *testing.T) {
	block := BeginMarker + "\nrimba content\n" + EndMarker

	t.Run("removes block preserving surrounding", func(t *testing.T) {
		content := "# Header\n\n" + block + "\n\n# Footer\n"
		result := removeBlock(content)
		if strings.Contains(result, BeginMarker) {
			t.Error("BEGIN marker should be removed")
		}
		if !strings.Contains(result, "Header") {
			t.Error("content before block should be preserved")
		}
		if !strings.Contains(result, "Footer") {
			t.Error("content after block should be preserved")
		}
	})

	t.Run("removes block at start", func(t *testing.T) {
		content := block + "\n"
		result := removeBlock(content)
		if result != "" {
			t.Errorf("expected empty result, got %q", result)
		}
	})

	t.Run("no markers returns unchanged", func(t *testing.T) {
		content := "just some text"
		result := removeBlock(content)
		if result != content {
			t.Errorf("expected unchanged content, got %q", result)
		}
	})

	t.Run("corrupt begin only", func(t *testing.T) {
		content := "# Header\n\n" + BeginMarker + "\ncorrupt"
		result := removeBlock(content)
		if strings.Contains(result, BeginMarker) {
			t.Error("BEGIN marker should be removed")
		}
		if !strings.Contains(result, "Header") {
			t.Error("content before block should be preserved")
		}
	})
}

// --- Content validation tests ---

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

// --- Error handling tests ---

func TestInstallReadError(t *testing.T) {
	dir := t.TempDir()

	// Create a directory named "AGENTS.md" so ReadFile fails
	agentsDir := filepath.Join(dir, "AGENTS.md")
	if err := os.Mkdir(agentsDir, 0755); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	_, err := Install(dir)
	if err == nil {
		t.Fatal("expected error when AGENTS.md is a directory")
	}
	if !strings.Contains(err.Error(), "read file") {
		t.Errorf("error = %q, want to contain 'read file'", err.Error())
	}
}

func TestInstallSkipsOnPathConflict(t *testing.T) {
	dir := t.TempDir()

	// Create .cursor as a file to block MkdirAll for nested paths
	blocker := filepath.Join(dir, ".cursor")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	// Install should succeed but skip .cursor/rules/rimba.mdc
	results, err := Install(dir)
	if err != nil {
		t.Fatalf("Install should succeed with path conflict, got: %v", err)
	}

	for _, r := range results {
		if r.RelPath == filepath.Join(".cursor", "rules", "rimba.mdc") {
			if r.Action != actionSkipped {
				t.Errorf(".cursor/rules/rimba.mdc action = %q, want %q", r.Action, actionSkipped)
			}
		}
	}
}

func TestUninstallReadError(t *testing.T) {
	dir := t.TempDir()

	// Create a directory named "AGENTS.md" so ReadFile fails
	agentsDir := filepath.Join(dir, "AGENTS.md")
	if err := os.Mkdir(agentsDir, 0755); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	_, err := Uninstall(dir)
	if err == nil {
		t.Fatal("expected error when AGENTS.md is a directory")
	}
	if !strings.Contains(err.Error(), "read file") {
		t.Errorf("error = %q, want to contain 'read file'", err.Error())
	}
}
