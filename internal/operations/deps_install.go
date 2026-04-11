package operations

import (
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/progress"
)

// DepsParams groups inputs for dependency detection and installation.
type DepsParams struct {
	WtPath        string
	Service       string
	AutoDetect    bool
	ConfigModules []config.ModuleConfig
	Entries       []git.WorktreeEntry
}

// InstallDeps detects modules and installs dependencies.
func InstallDeps(r git.Runner, p DepsParams, onProgress progress.Func) []deps.InstallResult {
	existingPaths := WorktreePathsExcluding(p.Entries, p.WtPath)

	modules, err := deps.ResolveModules(p.WtPath, p.Service, p.AutoDetect, p.ConfigModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return nil
	}

	mgr := &deps.Manager{Runner: r}
	return mgr.Install(p.WtPath, modules, p.Entries, onProgress)
}

// InstallDepsPreferSource is like InstallDeps but prefers cloning from sourceWT.
func InstallDepsPreferSource(r git.Runner, sourceWT string, p DepsParams, onProgress progress.Func) []deps.InstallResult {
	existingPaths := WorktreePathsExcluding(p.Entries, p.WtPath)

	modules, err := deps.ResolveModules(p.WtPath, p.Service, p.AutoDetect, p.ConfigModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return nil
	}

	mgr := &deps.Manager{Runner: r}
	return mgr.InstallPreferSource(p.WtPath, sourceWT, modules, p.Entries, onProgress)
}

// RunPostCreateHooks executes post-create hooks and returns the results.
func RunPostCreateHooks(wtPath string, hooks []string, onProgress progress.Func) []deps.HookResult {
	return deps.RunPostCreateHooks(wtPath, hooks, onProgress)
}

// WorktreePathsExcluding returns paths from entries, excluding the given path.
func WorktreePathsExcluding(entries []git.WorktreeEntry, exclude string) []string {
	var paths []string
	for _, e := range entries {
		if e.Path != exclude {
			paths = append(paths, e.Path)
		}
	}
	return paths
}
