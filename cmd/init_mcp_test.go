package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitGlobalRegistersMCP(t *testing.T) {
	home, restore := setupGlobalInit(t)
	defer restore()

	// Pre-seed .claude/settings.json so MCP registration doesn't skip it
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Agent files:") {
		t.Errorf("output should contain 'Agent files:', got:\n%s", out)
	}
	if !strings.Contains(out, "MCP server:") {
		t.Errorf("output should contain 'MCP server:', got:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(".claude", "settings.json")) {
		t.Errorf("output should mention settings.json, got:\n%s", out)
	}
	if !strings.Contains(out, "registered") {
		t.Errorf("output should say 'registered', got:\n%s", out)
	}

	// Verify the file was patched
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers missing in settings.json")
	}
	entry, ok := servers["rimba"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers.rimba missing")
	}
	if entry["command"] != "rimba" {
		t.Errorf("command = %v, want rimba", entry["command"])
	}
}

func TestInitGlobalMCPSkippedWhenNoConfig(t *testing.T) {
	_, restore := setupGlobalInit(t)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "skipped") {
		t.Errorf("output should contain 'skipped' for absent config files, got:\n%s", out)
	}
}

func TestInitGlobalUninstallRemovesMCP(t *testing.T) {
	home, restore := setupGlobalInit(t)
	defer restore()

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	// Install
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
	if !strings.Contains(out, "unregistered") {
		t.Errorf("output should say 'unregistered', got:\n%s", out)
	}

	// rimba key should be gone
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if servers, ok := cfg["mcpServers"].(map[string]any); ok {
		if _, found := servers["rimba"]; found {
			t.Error("rimba key should be removed after uninstall")
		}
	}
}

func TestInitAgentsMCPRegistration(t *testing.T) {
	repoDir := t.TempDir()

	// Pre-seed .mcp.json so registration doesn't skip
	if err := os.WriteFile(filepath.Join(repoDir, ".mcp.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatalf("write .mcp.json: %v", err)
	}

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagAgents, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "MCP server:") {
		t.Errorf("output should contain 'MCP server:', got:\n%s", out)
	}
	if !strings.Contains(out, ".mcp.json") {
		t.Errorf("output should mention .mcp.json, got:\n%s", out)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse .mcp.json: %v", err)
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers missing in .mcp.json")
	}
	if _, ok := servers["rimba"]; !ok {
		t.Error("mcpServers.rimba should be registered in .mcp.json")
	}
}

func TestInitAgentsLocalSkipsMCP(t *testing.T) {
	repoDir := t.TempDir()

	r := repoRootRunner(repoDir, func(args ...string) (string, error) {
		if args[0] == cmdSymbolicRef {
			return refsRemotesOriginMain, nil
		}
		return "", nil
	})
	restore := overrideNewRunner(r)
	defer restore()

	cmd, buf := newTestCmd()
	cmd.Flags().Bool(flagAgents, true, "")
	cmd.Flags().Bool(flagLocal, true, "")
	if err := initCmd.RunE(cmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "MCP server:") {
		t.Errorf("--agents --local should not output 'MCP server:' section, got:\n%s", out)
	}
}

func TestInitGlobalMCPErrorPropagated(t *testing.T) {
	home, restore := setupGlobalInit(t)
	defer restore()

	// Write malformed JSON to settings.json — triggers patchJSON error
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("not json"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cmd, _ := newTestCmd()
	cmd.Flags().Bool(flagGlobal, true, "")
	err := initCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for malformed MCP config, got nil")
	}
	if !strings.Contains(err.Error(), "mcp servers") {
		t.Errorf("error should mention 'mcp servers', got: %v", err)
	}
}
