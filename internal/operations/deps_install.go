package operations

import (
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
)

// InstallDeps detects modules and installs dependencies, returning the results.
// existingEntries should contain the current worktree list to avoid a redundant git call.
func InstallDeps(r git.Runner, wtPath string, autoDetect bool, configModules []config.ModuleConfig, existingEntries []git.WorktreeEntry, onProgress deps.ProgressFunc) []deps.InstallResult {
	existingPaths := WorktreePathsExcluding(existingEntries, wtPath)

	modules, err := deps.ResolveModules(wtPath, autoDetect, configModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return nil
	}

	mgr := &deps.Manager{Runner: r}
	return mgr.Install(wtPath, modules, onProgress)
}

// InstallDepsPreferSource is like InstallDeps but prefers cloning from sourceWT.
func InstallDepsPreferSource(r git.Runner, wtPath, sourceWT string, autoDetect bool, configModules []config.ModuleConfig, existingEntries []git.WorktreeEntry, onProgress deps.ProgressFunc) []deps.InstallResult {
	existingPaths := WorktreePathsExcluding(existingEntries, wtPath)

	modules, err := deps.ResolveModules(wtPath, autoDetect, configModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return nil
	}

	mgr := &deps.Manager{Runner: r}
	return mgr.InstallPreferSource(wtPath, sourceWT, modules, onProgress)
}

// RunPostCreateHooks executes post-create hooks and returns the results.
func RunPostCreateHooks(wtPath string, hooks []string, onProgress deps.ProgressFunc) []deps.HookResult {
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
