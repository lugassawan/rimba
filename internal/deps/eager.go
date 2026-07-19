package deps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
)

// resolveEagerness computes each module's Eager field: whether it installs
// automatically (true) or is deferred until explicitly requested (false).
// Precedence, highest wins: an explicit config override for that Dir; a
// service-scope match (the one module `service` specifically implies is
// needed); the heuristic default (Recursive => lazy, else eager).
func resolveEagerness(worktreePath, service string, modules []Module, configModules []config.ModuleConfig) []Module {
	configEager := make(map[string]bool, len(configModules))
	for _, cm := range configModules {
		if cm.Eager != nil {
			configEager[cm.Dir] = *cm.Eager
		}
	}

	targets := serviceScopeTargets(worktreePath, service, modules)

	for i := range modules {
		modules[i].Eager = eagerFor(modules[i], targets, configEager)
	}
	return modules
}

func eagerFor(mod Module, targets, configEager map[string]bool) bool {
	if override, ok := configEager[mod.Dir]; ok {
		return override
	}
	if targets[mod.Dir] {
		return true
	}
	return !mod.Recursive
}

// serviceScopeTargets returns the set of module Dirs that `service`
// specifically implies should be eager. Empty when service is "" or maps to
// nothing recognizable (e.g. a non-JS service, or a service rimba has no
// module for at all).
func serviceScopeTargets(worktreePath, service string, modules []Module) map[string]bool {
	targets := make(map[string]bool)
	if service == "" {
		return targets
	}

	// A detected module's own WorkDir equals service: a standalone-lockfile
	// service (its own package.json + lockfile, excluded from any workspace).
	for _, m := range modules {
		if m.WorkDir == service {
			targets[m.Dir] = true
		}
	}
	if len(targets) > 0 {
		return targets
	}

	// No standalone module for this service — if it has a package.json but
	// no lockfile of its own, it's a workspace member hoisted into a
	// Recursive parent module.
	if hasPackageJSON(worktreePath, service) {
		if dir := coveringRecursiveModuleDir(service, modules); dir != "" {
			targets[dir] = true
		}
	}
	return targets
}

func hasPackageJSON(worktreePath, service string) bool {
	info, err := os.Stat(filepath.Join(worktreePath, service, "package.json"))
	return err == nil && !info.IsDir()
}

// coveringRecursiveModuleDir finds the Recursive module whose WorkDir (""
// meaning repo root) is the longest matching ancestor of service. Returns ""
// if none covers it.
func coveringRecursiveModuleDir(service string, modules []Module) string {
	best, bestLen := "", -1
	for _, m := range modules {
		if !m.Recursive {
			continue
		}
		if m.WorkDir != "" && m.WorkDir != service && !strings.HasPrefix(service, m.WorkDir+"/") {
			continue
		}
		if len(m.WorkDir) > bestLen {
			best, bestLen = m.Dir, len(m.WorkDir)
		}
	}
	return best
}
