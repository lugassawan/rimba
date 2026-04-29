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

func TestSpecsReturnsSeven(t *testing.T) {
	if got := len(ProjectSpecs()); got != 7 {
		t.Fatalf("Specs() returned %d items, want 7", got)
	}
}

func TestSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range ProjectSpecs() {
		content := spec.Content()
		if content == "" {
			t.Errorf("Spec %q returned empty content", spec.RelPath)
		}
	}
}

// --- Install tests ---

func TestInstallCreatesAllFiles(t *testing.T) {
	dir := t.TempDir()

	results, err := InstallProject(dir)
	if err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if len(results) != 7 {
		t.Fatalf("Install returned %d results, want 7", len(results))
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

	if _, err := InstallProject(dir); err != nil {
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

	results, err := InstallProject(dir)
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

	if _, err := InstallProject(dir); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	// Second install should succeed (idempotent)
	results, err := InstallProject(dir)
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

	if _, err := InstallProject(dir); err != nil {
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

	if _, err := InstallProject(dir); err != nil {
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
	if _, err := InstallProject(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	results, err := UninstallProject(dir)
	if err != nil {
		t.Fatalf(fatalUninstall, err)
	}

	if len(results) != 7 {
		t.Fatalf("Uninstall returned %d results, want 7", len(results))
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

	if _, err := InstallProject(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if _, err := UninstallProject(dir); err != nil {
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
	if _, err := InstallProject(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	if _, err := UninstallProject(dir); err != nil {
		t.Fatalf(fatalUninstall, err)
	}

	// AGENTS.md should be deleted (no user content)
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Error("AGENTS.md should be deleted when only rimba content was present")
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	dir := t.TempDir()

	results, err := UninstallProject(dir)
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

	statuses := StatusProject(dir)
	if len(statuses) != 7 {
		t.Fatalf("Status returned %d items, want 7", len(statuses))
	}

	for _, s := range statuses {
		if s.Installed {
			t.Errorf("%s: expected Installed = false", s.RelPath)
		}
	}
}

func TestStatusInstalled(t *testing.T) {
	dir := t.TempDir()

	if _, err := InstallProject(dir); err != nil {
		t.Fatalf(fatalInstall, err)
	}

	statuses := StatusProject(dir)
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

	statuses := StatusProject(dir)
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

// --- New project-tier content tests ---

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

// --- Global (user-tier) content tests ---

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

// --- GlobalSpecs / ProjectSpecs tests ---

func TestGlobalSpecsCountIsSeven(t *testing.T) {
	if got := len(GlobalSpecs()); got != 7 {
		t.Fatalf("GlobalSpecs() returned %d items, want 7", got)
	}
}

func TestProjectSpecsCountIsSeven(t *testing.T) {
	if got := len(ProjectSpecs()); got != 7 {
		t.Fatalf("ProjectSpecs() returned %d items, want 7", got)
	}
}

func TestGlobalSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range GlobalSpecs() {
		if spec.Content() == "" {
			t.Errorf("GlobalSpec %q returned empty content", spec.RelPath)
		}
	}
}

func TestProjectSpecsContentNotEmpty(t *testing.T) {
	for _, spec := range ProjectSpecs() {
		if spec.Content() == "" {
			t.Errorf("ProjectSpec %q returned empty content", spec.RelPath)
		}
	}
}

// --- InstallGlobal / UninstallGlobal tests ---

func TestInstallGlobalCreatesAllFiles(t *testing.T) {
	home := t.TempDir()

	results, err := InstallGlobal(home)
	if err != nil {
		t.Fatalf("InstallGlobal: %v", err)
	}

	if len(results) != 7 {
		t.Fatalf("InstallGlobal returned %d results, want 7", len(results))
	}
	for _, r := range results {
		if r.Action != actionCreated {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionCreated)
		}
		path := filepath.Join(home, r.RelPath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("%s: file not created at %s", r.RelPath, path)
		}
	}
}

func TestInstallGlobalIdempotent(t *testing.T) {
	home := t.TempDir()

	if _, err := InstallGlobal(home); err != nil {
		t.Fatalf("first InstallGlobal: %v", err)
	}
	results, err := InstallGlobal(home)
	if err != nil {
		t.Fatalf("second InstallGlobal: %v", err)
	}
	for _, r := range results {
		if r.Action != actionUpdated {
			t.Errorf("%s: action = %q, want %q on re-install", r.RelPath, r.Action, actionUpdated)
		}
	}

	// Block files must have exactly one marker pair.
	for _, spec := range GlobalSpecs() {
		if spec.Kind != KindBlock {
			continue
		}
		path := filepath.Join(home, spec.RelPath)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", spec.RelPath, err)
		}
		s := string(data)
		if strings.Count(s, BeginMarker) != 1 {
			t.Errorf("%s: %d BEGIN markers, want 1", spec.RelPath, strings.Count(s, BeginMarker))
		}
	}
}

func TestGlobalBlockPreservesExistingUserContent(t *testing.T) {
	home := t.TempDir()

	// Pre-seed ~/.codex/AGENTS.md with user content.
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatal(err)
	}
	userContent := "# My Codex Config\n\nPersonal instructions.\n"
	if err := os.WriteFile(filepath.Join(codexDir, "AGENTS.md"), []byte(userContent), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallGlobal(home); err != nil {
		t.Fatalf("InstallGlobal: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(codexDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "My Codex Config") {
		t.Error("user content should be preserved")
	}
	if !strings.Contains(s, BeginMarker) {
		t.Error("rimba block should be appended")
	}
}

func TestUninstallGlobalRemovesAllFiles(t *testing.T) {
	home := t.TempDir()

	if _, err := InstallGlobal(home); err != nil {
		t.Fatalf("InstallGlobal: %v", err)
	}
	results, err := UninstallGlobal(home)
	if err != nil {
		t.Fatalf("UninstallGlobal: %v", err)
	}
	if len(results) != 7 {
		t.Fatalf("UninstallGlobal returned %d results, want 7", len(results))
	}
	for _, r := range results {
		if r.Action != actionRemoved {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionRemoved)
		}
	}
}

func TestStatusGlobalReflectsInstall(t *testing.T) {
	home := t.TempDir()

	before := StatusGlobal(home)
	for _, s := range before {
		if s.Installed {
			t.Errorf("%s: expected not installed before install", s.RelPath)
		}
	}

	if _, err := InstallGlobal(home); err != nil {
		t.Fatalf("InstallGlobal: %v", err)
	}

	after := StatusGlobal(home)
	for _, s := range after {
		if !s.Installed {
			t.Errorf("%s: expected installed after install", s.RelPath)
		}
	}
}

// --- InstallProject / UninstallProject tests ---

func TestInstallProjectCreatesAllFiles(t *testing.T) {
	dir := t.TempDir()

	results, err := InstallProject(dir)
	if err != nil {
		t.Fatalf("InstallProject: %v", err)
	}
	if len(results) != 7 {
		t.Fatalf("InstallProject returned %d results, want 7", len(results))
	}
	for _, r := range results {
		if r.Action != actionCreated {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionCreated)
		}
	}
}

func TestUninstallProjectRemovesFiles(t *testing.T) {
	dir := t.TempDir()

	if _, err := InstallProject(dir); err != nil {
		t.Fatalf("InstallProject: %v", err)
	}
	if _, err := UninstallProject(dir); err != nil {
		t.Fatalf("UninstallProject: %v", err)
	}

	// Whole-file types should be deleted.
	for _, spec := range ProjectSpecs() {
		if spec.Kind != KindWhole {
			continue
		}
		path := filepath.Join(dir, spec.RelPath)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s: should be deleted after uninstall", spec.RelPath)
		}
	}
}

// --- InstallLocal / UninstallLocal tests ---

func TestInstallLocalAddsGitignoreEntries(t *testing.T) {
	dir := t.TempDir()

	// Need a git repo-like directory; EnsureGitignore just needs the directory.
	if _, err := InstallLocal(dir); err != nil {
		t.Fatalf("InstallLocal: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	gitignore := string(data)

	for _, spec := range ProjectSpecs() {
		if !strings.Contains(gitignore, spec.RelPath) {
			t.Errorf(".gitignore missing entry for %s", spec.RelPath)
		}
	}
}

func TestUninstallLocalRemovesGitignoreEntries(t *testing.T) {
	dir := t.TempDir()

	if _, err := InstallLocal(dir); err != nil {
		t.Fatalf("InstallLocal: %v", err)
	}
	if _, err := UninstallLocal(dir); err != nil {
		t.Fatalf("UninstallLocal: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read .gitignore: %v", err)
	}
	gitignore := string(data)

	for _, spec := range ProjectSpecs() {
		if strings.Contains(gitignore, spec.RelPath) {
			t.Errorf(".gitignore still has entry for %s after uninstall", spec.RelPath)
		}
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

	_, err := InstallProject(dir)
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
	results, err := InstallProject(dir)
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

	_, err := UninstallProject(dir)
	if err == nil {
		t.Fatal("expected error when AGENTS.md is a directory")
	}
	if !strings.Contains(err.Error(), "read file") {
		t.Errorf("error = %q, want to contain 'read file'", err.Error())
	}
}

func TestInstallBlockSkipsOnPathConflict(t *testing.T) {
	dir := t.TempDir()

	// Create .github as a file to block MkdirAll for block-based copilot file
	blocker := filepath.Join(dir, ".github")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := InstallProject(dir)
	if err != nil {
		t.Fatalf("Install should succeed with block path conflict, got: %v", err)
	}

	for _, r := range results {
		if r.RelPath == filepath.Join(".github", "copilot-instructions.md") {
			if r.Action != actionSkipped {
				t.Errorf("copilot-instructions.md action = %q, want %q", r.Action, actionSkipped)
			}
		}
	}
}

func TestInstallWholeWriteError(t *testing.T) {
	dir := t.TempDir()

	// Create .cursor/rules/ directory then make it read-only
	rulesDir := filepath.Join(dir, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(rulesDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rulesDir, 0755) })

	// Install just a whole-file spec that targets .cursor/rules/rimba.mdc
	spec := Spec{
		RelPath: filepath.Join(".cursor", "rules", "rimba.mdc"),
		Kind:    KindWhole,
		Content: func() string { return "test" },
	}
	_, err := installOne(dir, spec)
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
	if !strings.Contains(err.Error(), "write file") {
		t.Errorf("error = %q, want to contain 'write file'", err.Error())
	}
}

func TestInstallBlockWriteError(t *testing.T) {
	dir := t.TempDir()

	// Make the directory read-only so WriteFile fails for AGENTS.md
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	spec := Spec{
		RelPath: "AGENTS.md",
		Kind:    KindBlock,
		Content: agentsBlock,
	}
	_, err := installOne(dir, spec)
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
	if !strings.Contains(err.Error(), "write file") {
		t.Errorf("error = %q, want to contain 'write file'", err.Error())
	}
}

func TestUninstallWholeRemoveError(t *testing.T) {
	dir := t.TempDir()

	// Create the file then make directory read-only
	skillDir := filepath.Join(dir, ".claude", "skills", "rimba")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(skillDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(skillDir, 0755) })

	spec := Spec{
		RelPath: filepath.Join(".claude", "skills", "rimba", "SKILL.md"),
		Kind:    KindWhole,
		Content: claudeSkillContent,
	}
	_, err := uninstallOne(dir, spec)
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
	if !strings.Contains(err.Error(), "remove file") {
		t.Errorf("error = %q, want to contain 'remove file'", err.Error())
	}
}

func TestUninstallBlockNoMarkers(t *testing.T) {
	dir := t.TempDir()

	// Create AGENTS.md without rimba markers
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# User content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	spec := Spec{
		RelPath: "AGENTS.md",
		Kind:    KindBlock,
		Content: agentsBlock,
	}
	r, err := uninstallOne(dir, spec)
	if err != nil {
		t.Fatalf("uninstallOne: %v", err)
	}
	if r.Action != actionSkipped {
		t.Errorf("action = %q, want %q", r.Action, actionSkipped)
	}
}

func TestUninstallBlockRemoveError(t *testing.T) {
	dir := t.TempDir()

	// Create AGENTS.md with only rimba block (will try to remove file)
	content := agentsBlock() + "\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Make directory read-only so os.Remove fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })

	spec := Spec{
		RelPath: "AGENTS.md",
		Kind:    KindBlock,
		Content: agentsBlock,
	}
	_, err := uninstallOne(dir, spec)
	if err == nil {
		t.Fatal("expected error when directory is read-only")
	}
	if !strings.Contains(err.Error(), "remove file") {
		t.Errorf("error = %q, want to contain 'remove file'", err.Error())
	}
}

func TestUninstallBlockWriteError(t *testing.T) {
	dir := t.TempDir()

	// Create AGENTS.md with user content + rimba block (will try to write cleaned content)
	content := "# User content\n\n" + agentsBlock() + "\n"
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Make file read-only so WriteFile fails
	if err := os.Chmod(agentsPath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(agentsPath, 0644) })

	spec := Spec{
		RelPath: "AGENTS.md",
		Kind:    KindBlock,
		Content: agentsBlock,
	}
	_, err := uninstallOne(dir, spec)
	if err == nil {
		t.Fatal("expected error when file is read-only")
	}
	if !strings.Contains(err.Error(), "write file") {
		t.Errorf("error = %q, want to contain 'write file'", err.Error())
	}
}

func TestRemoveBlockCorruptBeginAtStart(t *testing.T) {
	// BEGIN at the very start of content, no END marker
	content := BeginMarker + "\nsome corrupt content"
	result := removeBlock(content)
	if result != "" {
		t.Errorf("expected empty result when BEGIN is at start with no END, got %q", result)
	}
}
