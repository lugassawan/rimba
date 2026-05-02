package agentfile

import (
	"os"
	"path/filepath"
	"testing"
)

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
