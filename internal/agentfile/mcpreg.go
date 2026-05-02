package agentfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/pelletier/go-toml/v2"
)

const mcpServerName = "rimba"

type mcpFormat int

const (
	mcpJSON mcpFormat = iota
	mcpTOML
)

// mcpCodec parameterizes the unmarshal + write pair across JSON and TOML formats.
type mcpCodec struct {
	unmarshal func([]byte, any) error
	write     func(path string, cfg map[string]any) error
}

var (
	jsonCodec = mcpCodec{unmarshal: json.Unmarshal, write: writeJSON}
	tomlCodec = mcpCodec{unmarshal: toml.Unmarshal, write: writeTOML}
)

// MCPSpec describes a single agent MCP config file to patch.
type MCPSpec struct {
	RelPath      string    // relative to baseDir (homeDir or repoRoot)
	Format       mcpFormat // mcpJSON or mcpTOML
	ContainerKey string    // "mcpServers" for JSON agents, "mcp_servers" for Codex TOML
}

// GlobalMCPSpecs returns the MCP config files patched at user level (~/).
func GlobalMCPSpecs() []MCPSpec {
	return []MCPSpec{
		{RelPath: filepath.Join(".claude", "settings.json"), Format: mcpJSON, ContainerKey: "mcpServers"},
		{RelPath: filepath.Join(".cursor", "mcp.json"), Format: mcpJSON, ContainerKey: "mcpServers"},
		{RelPath: filepath.Join(".codeium", "windsurf", "mcp_config.json"), Format: mcpJSON, ContainerKey: "mcpServers"},
		{RelPath: filepath.Join(".codex", "config.toml"), Format: mcpTOML, ContainerKey: "mcp_servers"},
		{RelPath: filepath.Join(".gemini", "settings.json"), Format: mcpJSON, ContainerKey: "mcpServers"},
		{RelPath: filepath.Join(".roo", "mcp.json"), Format: mcpJSON, ContainerKey: "mcpServers"},
	}
}

// ProjectMCPSpecs returns the MCP config files patched at project level (repo root).
// .cursor/mcp.json is repo-root-relative (workspace MCP), distinct from ~/.cursor/mcp.json.
func ProjectMCPSpecs() []MCPSpec {
	return []MCPSpec{
		{RelPath: ".mcp.json", Format: mcpJSON, ContainerKey: "mcpServers"},
		{RelPath: filepath.Join(".cursor", "mcp.json"), Format: mcpJSON, ContainerKey: "mcpServers"},
	}
}

// RegisterMCPGlobal patches all user-level MCP config files to add the rimba server entry.
func RegisterMCPGlobal(homeDir string) ([]Result, error) {
	return applyMCPSpecs(homeDir, GlobalMCPSpecs(), false)
}

// UnregisterMCPGlobal removes the rimba server entry from all user-level MCP config files.
func UnregisterMCPGlobal(homeDir string) ([]Result, error) {
	return applyMCPSpecs(homeDir, GlobalMCPSpecs(), true)
}

// RegisterMCPProject patches project-level MCP config files to add the rimba server entry.
func RegisterMCPProject(repoRoot string) ([]Result, error) {
	return applyMCPSpecs(repoRoot, ProjectMCPSpecs(), false)
}

// UnregisterMCPProject removes the rimba server entry from project-level MCP config files.
func UnregisterMCPProject(repoRoot string) ([]Result, error) {
	return applyMCPSpecs(repoRoot, ProjectMCPSpecs(), true)
}

func codecFor(f mcpFormat) mcpCodec {
	if f == mcpTOML {
		return tomlCodec
	}
	return jsonCodec
}

func applyMCPSpecs(baseDir string, specs []MCPSpec, remove bool) ([]Result, error) {
	results := make([]Result, 0, len(specs))
	for _, spec := range specs {
		path := filepath.Join(baseDir, spec.RelPath)
		action, err := patchMCP(path, spec.ContainerKey, remove, codecFor(spec.Format))
		if err != nil {
			return results, err
		}
		results = append(results, Result{RelPath: spec.RelPath, Action: action})
	}
	return results, nil
}

func patchMCP(path, containerKey string, remove bool, codec mcpCodec) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return actionSkippedNoConfig, nil
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var cfg map[string]any
	if err := codec.unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	var action string
	if remove {
		action = removeFromContainer(cfg, containerKey)
	} else {
		action = addToContainer(cfg, containerKey)
	}
	if action == actionUnchanged {
		return action, nil
	}
	return action, codec.write(path, cfg)
}

func addToContainer(cfg map[string]any, containerKey string) string {
	servers, _ := cfg[containerKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	// desiredEntry uses []any; go-toml/v2 also decodes TOML arrays as []any,
	// so reflect.DeepEqual works correctly for TOML idempotency.
	desired := desiredEntry()
	if reflect.DeepEqual(servers[mcpServerName], desired) {
		return actionUnchanged
	}
	servers[mcpServerName] = desired
	cfg[containerKey] = servers
	return actionRegistered
}

func removeFromContainer(cfg map[string]any, containerKey string) string {
	servers, _ := cfg[containerKey].(map[string]any)
	if servers == nil {
		return actionUnchanged
	}
	if _, present := servers[mcpServerName]; !present {
		return actionUnchanged
	}
	delete(servers, mcpServerName)
	cfg[containerKey] = servers
	return actionUnregistered
}

func desiredEntry() map[string]any {
	return map[string]any{
		"command": mcpServerName,
		"args":    []any{"mcp"},
	}
}

func writeJSON(path string, cfg map[string]any) error {
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	out = append(out, '\n')
	return os.WriteFile(path, out, 0600)
}

func writeTOML(path string, cfg map[string]any) error {
	out, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return os.WriteFile(path, out, 0600)
}
