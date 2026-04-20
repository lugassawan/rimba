package agentfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// --- RegisterMCPGlobal tests ---

func TestRegisterMCPGlobalPatchesExistingJSON(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedJSON(t, filepath.Join(claudeDir, "settings.json"), map[string]any{"theme": "dark"})

	results, err := RegisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("RegisterMCPGlobal: %v", err)
	}

	claudeResult := findResult(t, results, filepath.Join(".claude", "settings.json"))
	if claudeResult.Action != actionRegistered {
		t.Errorf("action = %q, want %q", claudeResult.Action, actionRegistered)
	}

	cfg := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	if cfg["theme"] != "dark" {
		t.Error("existing 'theme' key was not preserved")
	}
	assertRimbaJSONEntry(t, cfg)
}

func TestRegisterMCPGlobalSkipsAbsentFiles(t *testing.T) {
	home := t.TempDir()

	results, err := RegisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("RegisterMCPGlobal: %v", err)
	}

	if len(results) != len(GlobalMCPSpecs()) {
		t.Fatalf("len(results) = %d, want %d", len(results), len(GlobalMCPSpecs()))
	}
	for _, r := range results {
		if r.Action != actionSkippedNoConfig {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionSkippedNoConfig)
		}
	}

	entries, _ := os.ReadDir(home)
	if len(entries) != 0 {
		t.Errorf("home dir should be empty but has %d entries", len(entries))
	}
}

func TestRegisterMCPGlobalIdempotent(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedJSON(t, filepath.Join(claudeDir, "settings.json"), map[string]any{})

	if _, err := RegisterMCPGlobal(home); err != nil {
		t.Fatalf("first RegisterMCPGlobal: %v", err)
	}
	firstContents, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	results, err := RegisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("second RegisterMCPGlobal: %v", err)
	}
	claudeResult := findResult(t, results, filepath.Join(".claude", "settings.json"))
	if claudeResult.Action != actionUnchanged {
		t.Errorf("second register: action = %q, want %q", claudeResult.Action, actionUnchanged)
	}

	secondContents, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(firstContents) != string(secondContents) {
		t.Error("file changed on idempotent re-registration")
	}
}

func TestRegisterMCPGlobalTOMLCodex(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := "[mcp_servers.other]\ncommand = \"x\"\nargs = []\n"
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(existing), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	results, err := RegisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("RegisterMCPGlobal: %v", err)
	}

	codexResult := findResult(t, results, filepath.Join(".codex", "config.toml"))
	if codexResult.Action != actionRegistered {
		t.Errorf("action = %q, want %q", codexResult.Action, actionRegistered)
	}

	data, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg map[string]any
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("toml unmarshal: %v", err)
	}
	servers, ok := cfg["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatal("mcp_servers missing or wrong type")
	}
	if _, ok := servers["other"]; !ok {
		t.Error("existing 'other' entry was removed")
	}
	entry, ok := servers[mcpServerName].(map[string]any)
	if !ok {
		t.Fatal("mcp_servers.rimba missing or wrong type")
	}
	if entry["command"] != mcpServerName {
		t.Errorf("command = %v, want %s", entry["command"], mcpServerName)
	}

	// Idempotency: second registration must return actionUnchanged for TOML
	results2, err := RegisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("second RegisterMCPGlobal: %v", err)
	}
	codexResult2 := findResult(t, results2, filepath.Join(".codex", "config.toml"))
	if codexResult2.Action != actionUnchanged {
		t.Errorf("second register (TOML): action = %q, want %q", codexResult2.Action, actionUnchanged)
	}
}

// --- UnregisterMCPGlobal tests ---

func TestUnregisterMCPGlobalRemovesOnlyRimba(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedJSON(t, filepath.Join(claudeDir, "settings.json"), map[string]any{
		"mcpServers": map[string]any{
			mcpServerName: map[string]any{"command": mcpServerName, "args": []any{"mcp"}},
			"other":       map[string]any{"command": "other-tool", "args": []any{}},
		},
	})

	results, err := UnregisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("UnregisterMCPGlobal: %v", err)
	}

	claudeResult := findResult(t, results, filepath.Join(".claude", "settings.json"))
	if claudeResult.Action != actionUnregistered {
		t.Errorf("action = %q, want %q", claudeResult.Action, actionUnregistered)
	}

	cfg := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers should still be present")
	}
	if _, found := servers[mcpServerName]; found {
		t.Error("rimba entry should have been removed")
	}
	if _, found := servers["other"]; !found {
		t.Error("other entry should be preserved")
	}
}

func TestUnregisterMCPGlobalSkipsAbsent(t *testing.T) {
	home := t.TempDir()

	results, err := UnregisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("UnregisterMCPGlobal: %v", err)
	}

	for _, r := range results {
		if r.Action != actionSkippedNoConfig {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionSkippedNoConfig)
		}
	}
}

func TestUnregisterMCPGlobalSkipsWhenKeyMissing(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedJSON(t, filepath.Join(claudeDir, "settings.json"), map[string]any{"theme": "dark"})

	results, err := UnregisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("UnregisterMCPGlobal: %v", err)
	}

	claudeResult := findResult(t, results, filepath.Join(".claude", "settings.json"))
	if claudeResult.Action != actionUnchanged {
		t.Errorf("action = %q, want %q", claudeResult.Action, actionUnchanged)
	}
}

// --- RegisterMCPProject tests ---

func TestRegisterMCPProjectPatchesDotMcpJson(t *testing.T) {
	repoRoot := t.TempDir()
	seedJSON(t, filepath.Join(repoRoot, ".mcp.json"), map[string]any{})
	cursorDir := filepath.Join(repoRoot, ".cursor")
	if err := os.MkdirAll(cursorDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedJSON(t, filepath.Join(cursorDir, "mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other", "args": []any{}},
		},
	})

	results, err := RegisterMCPProject(repoRoot)
	if err != nil {
		t.Fatalf("RegisterMCPProject: %v", err)
	}

	if len(results) != len(ProjectMCPSpecs()) {
		t.Fatalf("len(results) = %d, want %d", len(results), len(ProjectMCPSpecs()))
	}
	for _, r := range results {
		if r.Action != actionRegistered {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionRegistered)
		}
	}

	assertRimbaJSONEntry(t, readJSONFile(t, filepath.Join(repoRoot, ".mcp.json")))

	cursorCfg := readJSONFile(t, filepath.Join(cursorDir, "mcp.json"))
	assertRimbaJSONEntry(t, cursorCfg)
	servers, ok := cursorCfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers missing in .cursor/mcp.json")
	}
	if _, ok := servers["other"]; !ok {
		t.Error(".cursor/mcp.json: 'other' entry was removed")
	}
}

func TestRegisterMCPProjectSkipsAbsentFiles(t *testing.T) {
	repoRoot := t.TempDir()

	results, err := RegisterMCPProject(repoRoot)
	if err != nil {
		t.Fatalf("RegisterMCPProject: %v", err)
	}

	for _, r := range results {
		if r.Action != actionSkippedNoConfig {
			t.Errorf("%s: action = %q, want %q", r.RelPath, r.Action, actionSkippedNoConfig)
		}
	}
}

func TestUnregisterMCPGlobalTOMLCodex(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Seed with rimba and other entries
	content := "[mcp_servers.rimba]\ncommand = \"rimba\"\nargs = [\"mcp\"]\n[mcp_servers.other]\ncommand = \"x\"\nargs = []\n"
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(content), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	results, err := UnregisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("UnregisterMCPGlobal: %v", err)
	}

	codexResult := findResult(t, results, filepath.Join(".codex", "config.toml"))
	if codexResult.Action != actionUnregistered {
		t.Errorf("action = %q, want %q", codexResult.Action, actionUnregistered)
	}

	data, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg map[string]any
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("toml unmarshal: %v", err)
	}
	servers, ok := cfg["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatal("mcp_servers should still exist")
	}
	if _, found := servers[mcpServerName]; found {
		t.Error("rimba entry should have been removed from TOML config")
	}
	if _, found := servers["other"]; !found {
		t.Error("other entry should be preserved in TOML config")
	}
}

func TestPatchTOMLMalformedReturnsError(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("not = [valid toml\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := RegisterMCPGlobal(home)
	if err == nil {
		t.Error("expected error for malformed TOML, got nil")
	}
	if !strings.Contains(err.Error(), "config.toml") {
		t.Errorf("error should mention config.toml, got: %v", err)
	}
}

func TestPatchJSONMalformedReturnsError(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("not json {{{"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := RegisterMCPGlobal(home)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "settings.json") {
		t.Errorf("error should mention the file path, got: %v", err)
	}
}

// --- UnregisterMCPProject tests ---

func TestUnregisterMCPProjectRemovesRimba(t *testing.T) {
	repoRoot := t.TempDir()
	seedJSON(t, filepath.Join(repoRoot, ".mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			mcpServerName: map[string]any{"command": mcpServerName, "args": []any{"mcp"}},
		},
	})

	results, err := UnregisterMCPProject(repoRoot)
	if err != nil {
		t.Fatalf("UnregisterMCPProject: %v", err)
	}

	var mcpResult *Result
	for i := range results {
		if results[i].RelPath == ".mcp.json" {
			mcpResult = &results[i]
			break
		}
	}
	if mcpResult == nil {
		t.Fatal("no result for .mcp.json")
	}
	if mcpResult.Action != actionUnregistered {
		t.Errorf("action = %q, want %q", mcpResult.Action, actionUnregistered)
	}

	cfg := readJSONFile(t, filepath.Join(repoRoot, ".mcp.json"))
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers should still exist")
	}
	if _, found := servers[mcpServerName]; found {
		t.Error("rimba should have been removed")
	}
}

func TestRegisterMCPGlobalTOMLNoExistingSection(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// TOML file with no mcp_servers section — triggers addToTOML nil branch
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("[settings]\nfoo = \"bar\"\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	results, err := RegisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("RegisterMCPGlobal: %v", err)
	}

	codexResult := findResult(t, results, filepath.Join(".codex", "config.toml"))
	if codexResult.Action != actionRegistered {
		t.Errorf("action = %q, want %q", codexResult.Action, actionRegistered)
	}

	data, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg map[string]any
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("toml unmarshal: %v", err)
	}
	servers, ok := cfg["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatal("mcp_servers should have been created")
	}
	if _, ok := servers[mcpServerName]; !ok {
		t.Error("rimba entry should have been added")
	}
}

func TestUnregisterMCPGlobalTOMLSkipsWhenNoSection(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// TOML file with no mcp_servers section — triggers removeFromTOML nil branch
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("[settings]\nfoo = \"bar\"\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	results, err := UnregisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("UnregisterMCPGlobal: %v", err)
	}

	codexResult := findResult(t, results, filepath.Join(".codex", "config.toml"))
	if codexResult.Action != actionUnchanged {
		t.Errorf("action = %q, want %q", codexResult.Action, actionUnchanged)
	}
}

func TestUnregisterMCPGlobalTOMLSkipsWhenKeyMissing(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// mcp_servers section exists but no rimba key
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("[mcp_servers.other]\ncommand = \"x\"\nargs = []\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	results, err := UnregisterMCPGlobal(home)
	if err != nil {
		t.Fatalf("UnregisterMCPGlobal: %v", err)
	}

	codexResult := findResult(t, results, filepath.Join(".codex", "config.toml"))
	if codexResult.Action != actionUnchanged {
		t.Errorf("action = %q, want %q", codexResult.Action, actionUnchanged)
	}
}

func TestUnregisterMCPGlobalMalformedTOMLReturnsError(t *testing.T) {
	home := t.TempDir()
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte("not = [valid toml\n"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := UnregisterMCPGlobal(home)
	if err == nil {
		t.Error("expected error for malformed TOML on unregister, got nil")
	}
	if !strings.Contains(err.Error(), "config.toml") {
		t.Errorf("error should mention config.toml, got: %v", err)
	}
}

// --- helpers ---

func seedJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return v
}

func assertRimbaJSONEntry(t *testing.T, cfg map[string]any) {
	t.Helper()
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers key missing or wrong type")
	}
	entry, ok := servers[mcpServerName].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers.%s missing or wrong type", mcpServerName)
	}
	if entry["command"] != mcpServerName {
		t.Errorf("command = %v, want %s", entry["command"], mcpServerName)
	}
	args, ok := entry["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "mcp" {
		t.Errorf("args = %v, want [mcp]", entry["args"])
	}
}

func findResult(t *testing.T, results []Result, relPath string) Result {
	t.Helper()
	for _, r := range results {
		if r.RelPath == relPath {
			return r
		}
	}
	t.Fatalf("no result for %s", relPath)
	return Result{}
}
