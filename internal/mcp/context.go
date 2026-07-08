package mcp

import (
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
)

// HandlerContext holds shared dependencies for MCP tool handlers.
// Created once in cmd/mcp.go, captured by handler closures.
type HandlerContext struct {
	Runner   git.Runner
	GH       gh.Runner      // nil unless PR operations are in scope; set to gh.Default() in production
	Config   *config.Config // may be nil if not in a rimba-initialized repo
	RepoRoot string
	Version  string
}

// PrefixSet resolves the resolver.PrefixSet for this handler context: the
// configured custom prefixes merged with the built-ins, or just the
// built-ins when no config is available. Never returns nil, mirroring
// config.PrefixSetFromContext's contract.
func (h *HandlerContext) PrefixSet() *resolver.PrefixSet {
	if h.Config == nil {
		return resolver.DefaultPrefixSet()
	}
	return h.Config.PrefixSet()
}

// requireConfig returns the config or an error if not available.
func (h *HandlerContext) requireConfig() (*config.Config, error) {
	if h.Config == nil {
		return nil, errConfigRequired
	}
	return h.Config, nil
}
