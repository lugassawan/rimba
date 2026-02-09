package cmd

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
)

// printDepsResults detects modules, installs deps, and prints the results.
func printDepsResults(out io.Writer, r git.Runner, cfg *config.Config, wtPath string) {
	var configModules []config.ModuleConfig
	if cfg.Deps != nil {
		configModules = cfg.Deps.Modules
	}

	// Get existing worktree paths for FilterCloneOnly
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return
	}
	var existingPaths []string
	for _, e := range entries {
		if e.Path != wtPath {
			existingPaths = append(existingPaths, e.Path)
		}
	}

	modules, err := deps.ResolveModules(wtPath, cfg.IsAutoDetectDeps(), configModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return
	}

	mgr := &deps.Manager{Runner: r}
	results := mgr.Install(wtPath, modules)
	printInstallResults(out, results)
}

// printDepsResultsPreferSource is like printDepsResults but prefers cloning from sourceWT.
func printDepsResultsPreferSource(out io.Writer, r git.Runner, cfg *config.Config, wtPath, sourceWT string) {
	var configModules []config.ModuleConfig
	if cfg.Deps != nil {
		configModules = cfg.Deps.Modules
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return
	}
	var existingPaths []string
	for _, e := range entries {
		if e.Path != wtPath {
			existingPaths = append(existingPaths, e.Path)
		}
	}

	modules, err := deps.ResolveModules(wtPath, cfg.IsAutoDetectDeps(), configModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return
	}

	mgr := &deps.Manager{Runner: r}
	results := mgr.InstallPreferSource(wtPath, sourceWT, modules)
	printInstallResults(out, results)
}

func printInstallResults(out io.Writer, results []deps.InstallResult) {
	var printed bool
	for _, r := range results {
		if !r.Cloned && r.Error == nil {
			continue
		}
		if !printed {
			fmt.Fprintf(out, "  Dependencies:\n")
			printed = true
		}
		if r.Cloned {
			fmt.Fprintf(out, "    %s: cloned from %s\n", r.Module.Dir, filepath.Base(r.Source))
		} else if r.Error != nil {
			fmt.Fprintf(out, "    %s: %v\n", r.Module.Dir, r.Error)
		}
	}
}

// printHookResults runs post-create hooks and prints their results.
func printHookResults(out io.Writer, wtPath string, hooks []string) {
	results := deps.RunPostCreateHooks(wtPath, hooks)
	fmt.Fprintf(out, "  Hooks:\n")
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(out, "    %s: %v\n", r.Command, r.Error)
		} else {
			fmt.Fprintf(out, "    %s: ok\n", r.Command)
		}
	}
}
