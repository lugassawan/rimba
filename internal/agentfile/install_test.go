package agentfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
