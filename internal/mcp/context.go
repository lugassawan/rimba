package mcp

import (
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
)

// HandlerContext holds shared dependencies for MCP tool handlers.
// Created once in cmd/mcp.go, captured by handler closures.
type HandlerContext struct {
	Runner   git.Runner
	Config   *config.Config // may be nil if not in a rimba-initialized repo
	RepoRoot string
	Version  string
}

// requireConfig returns the config or an error if not available.
func (h *HandlerContext) requireConfig() (*config.Config, error) {
	if h.Config == nil {
		return nil, errConfigRequired
	}
	return h.Config, nil
}
