package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitGlobalInstall(t *testing.T) {
	home, restore := setupGlobalInit(t)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "user") {
		t.Errorf("output should mention 'user', got:\n%s", out)
	}

	// Verify Claude skill was created
	claudePath := filepath.Join(home, ".claude", "skills", "rimba", "SKILL.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		t.Errorf("global claude skill not created at %s", claudePath)
	}
}

func TestInitGlobalNoRepoRequired(t *testing.T) {
	_, restore := setupGlobalInit(t)
	defer restore()

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")

	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("-g should succeed outside a git repo, got: %v", err)
	}
}

func TestInitGlobalUninstall(t *testing.T) {
	home, restore := setupGlobalInit(t)
	defer restore()

	cmd1, _ := newTestCmd()
	cmd1.Flags().Bool(flagGlobal, true, "")
	if err := initCmd.RunE(cmd1, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Uninstall
	cmd2, buf := newTestCmd()
	cmd2.Flags().Bool(flagGlobal, true, "")
	cmd2.Flags().Bool(flagUninstall, true, "")
	if err := initCmd.RunE(cmd2, nil); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Removed") {
		t.Errorf("output should say 'Removed', got:\n%s", out)
	}

	// Claude skill should be gone
	claudePath := filepath.Join(home, ".claude", "skills", "rimba", "SKILL.md")
	if _, err := os.Stat(claudePath); err == nil {
		t.Error("claude skill should be removed after uninstall")
	}
}
