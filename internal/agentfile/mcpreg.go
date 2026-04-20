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
	return registerMCPSpecs(homeDir, GlobalMCPSpecs())
}

// UnregisterMCPGlobal removes the rimba server entry from all user-level MCP config files.
func UnregisterMCPGlobal(homeDir string) ([]Result, error) {
	return unregisterMCPSpecs(homeDir, GlobalMCPSpecs())
}

// RegisterMCPProject patches project-level MCP config files to add the rimba server entry.
func RegisterMCPProject(repoRoot string) ([]Result, error) {
	return registerMCPSpecs(repoRoot, ProjectMCPSpecs())
}

// UnregisterMCPProject removes the rimba server entry from project-level MCP config files.
func UnregisterMCPProject(repoRoot string) ([]Result, error) {
	return unregisterMCPSpecs(repoRoot, ProjectMCPSpecs())
}

func registerMCPSpecs(baseDir string, specs []MCPSpec) ([]Result, error) {
	results := make([]Result, 0, len(specs))
	for _, spec := range specs {
		path := filepath.Join(baseDir, spec.RelPath)
		var action string
		var err error
		switch spec.Format {
		case mcpJSON:
			action, err = patchJSON(path, spec.ContainerKey, false)
		case mcpTOML:
			action, err = patchTOML(path, spec.ContainerKey, false)
		}
		if err != nil {
			return results, err
		}
		results = append(results, Result{RelPath: spec.RelPath, Action: action})
	}
	return results, nil
}

func unregisterMCPSpecs(baseDir string, specs []MCPSpec) ([]Result, error) {
	results := make([]Result, 0, len(specs))
	for _, spec := range specs {
		path := filepath.Join(baseDir, spec.RelPath)
		var action string
		var err error
		switch spec.Format {
		case mcpJSON:
			action, err = patchJSON(path, spec.ContainerKey, true)
		case mcpTOML:
			action, err = patchTOML(path, spec.ContainerKey, true)
		}
		if err != nil {
			return results, err
		}
		results = append(results, Result{RelPath: spec.RelPath, Action: action})
	}
	return results, nil
}

func desiredEntry() map[string]any {
	return map[string]any{
		"command": mcpServerName,
		"args":    []any{"mcp"},
	}
}

func patchJSON(path, containerKey string, remove bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return actionSkippedNoConfig, nil
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if remove {
		return removeFromJSON(path, containerKey, cfg)
	}
	return addToJSON(path, containerKey, cfg)
}

func addToJSON(path, containerKey string, cfg map[string]any) (string, error) {
	servers, _ := cfg[containerKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	desired := desiredEntry()
	if reflect.DeepEqual(servers[mcpServerName], desired) {
		return actionUnchanged, nil
	}
	servers[mcpServerName] = desired
	cfg[containerKey] = servers
	return actionRegistered, writeJSON(path, cfg)
}

func removeFromJSON(path, containerKey string, cfg map[string]any) (string, error) {
	servers, _ := cfg[containerKey].(map[string]any)
	if servers == nil {
		return actionUnchanged, nil
	}
	if _, present := servers[mcpServerName]; !present {
		return actionUnchanged, nil
	}
	delete(servers, mcpServerName)
	cfg[containerKey] = servers
	return actionUnregistered, writeJSON(path, cfg)
}

func patchTOML(path, containerKey string, remove bool) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return actionSkippedNoConfig, nil
		}
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var cfg map[string]any
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	if remove {
		return removeFromTOML(path, containerKey, cfg)
	}
	return addToTOML(path, containerKey, cfg)
}

func addToTOML(path, containerKey string, cfg map[string]any) (string, error) {
	servers, _ := cfg[containerKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	// TOML library decodes args as []any, matching desiredEntry() shape.
	desired := desiredEntry()
	if reflect.DeepEqual(servers[mcpServerName], desired) {
		return actionUnchanged, nil
	}
	servers[mcpServerName] = desired
	cfg[containerKey] = servers
	return actionRegistered, writeTOML(path, cfg)
}

func removeFromTOML(path, containerKey string, cfg map[string]any) (string, error) {
	servers, _ := cfg[containerKey].(map[string]any)
	if servers == nil {
		return actionUnchanged, nil
	}
	if _, present := servers[mcpServerName]; !present {
		return actionUnchanged, nil
	}
	delete(servers, mcpServerName)
	cfg[containerKey] = servers
	return actionUnregistered, writeTOML(path, cfg)
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
