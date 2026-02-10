package cmd

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
)

// installDeps detects modules and installs dependencies, returning the results.
func installDeps(r git.Runner, cfg *config.Config, wtPath string) []deps.InstallResult {
	var configModules []config.ModuleConfig
	if cfg.Deps != nil {
		configModules = cfg.Deps.Modules
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil
	}
	var existingPaths []string
	for _, e := range entries {
		if e.Path != wtPath {
			existingPaths = append(existingPaths, e.Path)
		}
	}

	modules, err := deps.ResolveModules(wtPath, cfg.IsAutoDetectDeps(), configModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return nil
	}

	mgr := &deps.Manager{Runner: r}
	return mgr.Install(wtPath, modules)
}

// installDepsPreferSource is like installDeps but prefers cloning from sourceWT.
func installDepsPreferSource(r git.Runner, cfg *config.Config, wtPath, sourceWT string) []deps.InstallResult {
	var configModules []config.ModuleConfig
	if cfg.Deps != nil {
		configModules = cfg.Deps.Modules
	}

	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil
	}
	var existingPaths []string
	for _, e := range entries {
		if e.Path != wtPath {
			existingPaths = append(existingPaths, e.Path)
		}
	}

	modules, err := deps.ResolveModules(wtPath, cfg.IsAutoDetectDeps(), configModules, existingPaths)
	if err != nil || len(modules) == 0 {
		return nil
	}

	mgr := &deps.Manager{Runner: r}
	return mgr.InstallPreferSource(wtPath, sourceWT, modules)
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

// runHooks executes post-create hooks and returns the results.
func runHooks(wtPath string, hooks []string) []deps.HookResult {
	return deps.RunPostCreateHooks(wtPath, hooks)
}

// printHookResultsList prints pre-computed hook results.
func printHookResultsList(out io.Writer, results []deps.HookResult) {
	if len(results) == 0 {
		return
	}
	fmt.Fprintf(out, "  Hooks:\n")
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(out, "    %s: %v\n", r.Command, r.Error)
		} else {
			fmt.Fprintf(out, "    %s: ok\n", r.Command)
		}
	}
}
