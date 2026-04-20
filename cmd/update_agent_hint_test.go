package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lugassawan/rimba/internal/agentfile"
)

func TestAnyInstalled(t *testing.T) {
	tests := []struct {
		name     string
		statuses []agentfile.FileStatus
		want     bool
	}{
		{"empty slice", nil, false},
		{"none installed", []agentfile.FileStatus{{RelPath: "a", Installed: false}}, false},
		{"one installed", []agentfile.FileStatus{{RelPath: "a", Installed: true}}, true},
		{"mixed, last installed", []agentfile.FileStatus{{RelPath: "a", Installed: false}, {RelPath: "b", Installed: true}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := anyInstalled(tt.statuses); got != tt.want {
				t.Errorf("anyInstalled(%v) = %v, want %v", tt.statuses, got, tt.want)
			}
		})
	}
}

// createUserTierFile creates the sentinel KindWhole file that StatusGlobal detects.
func createUserTierFile(t *testing.T, homeDir string) {
	t.Helper()
	path := filepath.Join(homeDir, ".claude", "skills", "rimba", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("skill"), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// createProjectTierFile creates the sentinel KindBlock file that StatusProject detects.
// AGENTS.md must contain both BEGIN and END RIMBA markers to register as installed.
func createProjectTierFile(t *testing.T, repoRoot string) {
	t.Helper()
	path := filepath.Join(repoRoot, "AGENTS.md")
	content := agentfile.BeginMarker + "\n" + agentfile.EndMarker
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestAgentRefreshTipsUserInstalled(t *testing.T) {
	home := t.TempDir()
	createUserTierFile(t, home)
	cmd, buf := newTestCmd()

	printAgentRefreshTips(cmd, home, "")

	out := buf.String()
	if !strings.Contains(out, "user level") {
		t.Errorf("output missing 'user level': %q", out)
	}
	if !strings.Contains(out, "rimba init -g") {
		t.Errorf("output missing 'rimba init -g': %q", out)
	}
	if strings.Contains(out, "in this repo") {
		t.Errorf("unexpected project tip with empty repoRoot: %q", out)
	}
}

func TestAgentRefreshTipsProjectInstalled(t *testing.T) {
	repo := t.TempDir()
	createProjectTierFile(t, repo)
	cmd, buf := newTestCmd()

	printAgentRefreshTips(cmd, "", repo)

	out := buf.String()
	if !strings.Contains(out, "in this repo") {
		t.Errorf("output missing 'in this repo': %q", out)
	}
	if !strings.Contains(out, "rimba init --agents") {
		t.Errorf("output missing 'rimba init --agents': %q", out)
	}
	if strings.Contains(out, "user level") {
		t.Errorf("unexpected user tip with empty home: %q", out)
	}
}

func TestAgentRefreshTipsBothInstalled(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	createUserTierFile(t, home)
	createProjectTierFile(t, repo)
	cmd, buf := newTestCmd()

	printAgentRefreshTips(cmd, home, repo)

	out := buf.String()
	if !strings.Contains(out, "user level") {
		t.Errorf("output missing 'user level': %q", out)
	}
	if !strings.Contains(out, "in this repo") {
		t.Errorf("output missing 'in this repo': %q", out)
	}
}

func TestAgentRefreshTipsNothingInstalled(t *testing.T) {
	cmd, buf := newTestCmd()

	printAgentRefreshTips(cmd, t.TempDir(), t.TempDir())

	if buf.Len() != 0 {
		t.Errorf("expected empty output when nothing installed, got %q", buf.String())
	}
}

func TestAgentRefreshTipsRimbaQuiet(t *testing.T) {
	t.Setenv("RIMBA_QUIET", "1")
	home := t.TempDir()
	repo := t.TempDir()
	createUserTierFile(t, home)
	createProjectTierFile(t, repo)
	cmd, buf := newTestCmd()

	printAgentRefreshTips(cmd, home, repo)

	if buf.Len() != 0 {
		t.Errorf("expected empty output with RIMBA_QUIET=1, got %q", buf.String())
	}
}

func TestAgentRefreshTipsEmptyPaths(t *testing.T) {
	cmd, buf := newTestCmd()

	printAgentRefreshTips(cmd, "", "")

	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty paths, got %q", buf.String())
	}
}
