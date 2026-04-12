package deps

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/debug"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/parallel"
	"github.com/lugassawan/rimba/internal/progress"
)

// defaultDepsConcurrencyCap bounds the auto-selected worker pool.
// Package-manager global stores (pnpm, npm) serialize on their own locks,
// so going wider than this rarely helps and can starve CPU on small machines.
const defaultDepsConcurrencyCap = 4

// Manager orchestrates dependency installation for worktrees.
type Manager struct {
	Runner git.Runner

	// Concurrency caps parallel module installs. <= 0 auto-picks a default.
	Concurrency int
}

// InstallResult holds the outcome of installing a single module.
type InstallResult struct {
	Module Module
	Source string // source worktree path if cloned
	Cloned bool
	Error  error
}

// Install clones or installs deps for each module.
// Pass existingEntries to skip an extra git.ListWorktrees call; nil fetches its own.
func (m *Manager) Install(worktreePath string, modules []Module, existingEntries []git.WorktreeEntry, onProgress progress.Func) []InstallResult {
	return m.install(worktreePath, "", modules, existingEntries, onProgress)
}

// InstallPreferSource is like Install but tries sourceWT first when cloning.
func (m *Manager) InstallPreferSource(worktreePath, sourceWT string, modules []Module, existingEntries []git.WorktreeEntry, onProgress progress.Func) []InstallResult {
	return m.install(worktreePath, sourceWT, modules, existingEntries, onProgress)
}

// ResolveModules detects and merges modules, filtering clone-only ones.
func ResolveModules(worktreePath, service string, autoDetect bool, configModules []config.ModuleConfig, existingWTPaths []string) ([]Module, error) {
	var modules []Module

	if autoDetect {
		detected, err := DetectModules(worktreePath, service)
		if err != nil {
			return nil, err
		}
		modules = MergeWithConfig(detected, configModules)
	} else if len(configModules) > 0 {
		for _, cm := range configModules {
			modules = append(modules, moduleFromConfig(cm))
		}
	}

	if len(modules) == 0 {
		return nil, nil
	}

	modules = FilterCloneOnly(modules, existingWTPaths)

	return modules, nil
}

func (m *Manager) install(worktreePath, sourceWT string, modules []Module, existingEntries []git.WorktreeEntry, onProgress progress.Func) []InstallResult {
	defer debug.StartTimer("installing dependencies")()
	results := make([]InstallResult, 0, len(modules))

	hashed, err := HashModules(worktreePath, modules)
	if err != nil {
		for _, mod := range modules {
			results = append(results, InstallResult{Module: mod, Error: err})
		}
		return results
	}

	entries := existingEntries
	if entries == nil {
		entries, err = git.ListWorktrees(m.Runner)
		if err != nil {
			for _, mod := range modules {
				results = append(results, InstallResult{Module: mod, Error: err})
			}
			return results
		}
	}

	existingPaths := buildExistingPaths(entries, worktreePath, sourceWT)

	concurrency := m.resolveConcurrency()
	var done atomic.Int32
	total := len(hashed)

	results = parallel.Collect(total, concurrency, func(i int) InstallResult {
		res := m.installModule(worktreePath, hashed[i], existingPaths)
		completed := done.Add(1)
		progress.Notifyf(onProgress, "%d/%d complete", completed, total)
		return res
	})

	return results
}

func (m *Manager) resolveConcurrency() int {
	if m.Concurrency >= 1 {
		return m.Concurrency
	}
	return max(1, min(runtime.NumCPU(), defaultDepsConcurrencyCap))
}

func buildExistingPaths(entries []git.WorktreeEntry, exclude, preferred string) []string {
	var paths []string
	if preferred != "" {
		paths = append(paths, preferred)
	}
	for _, e := range entries {
		if e.Path != exclude && e.Path != preferred {
			paths = append(paths, e.Path)
		}
	}
	return paths
}

func (m *Manager) installModule(worktreePath string, mh ModuleWithHash, existingPaths []string) InstallResult {
	mod := mh.Module

	if mh.Hash == "" {
		return InstallResult{Module: mod}
	}

	if result, ok := tryCloneFromExisting(worktreePath, mh, existingPaths); ok {
		return result
	}

	if mod.CloneOnly {
		return InstallResult{Module: mod}
	}

	if mod.InstallCmd != "" {
		err := runInstall(worktreePath, mod)
		return InstallResult{Module: mod, Error: err}
	}

	return InstallResult{Module: mod}
}

// tryCloneFromExisting attempts to clone the module from an existing worktree with matching lockfile.
func tryCloneFromExisting(worktreePath string, mh ModuleWithHash, existingPaths []string) (InstallResult, bool) {
	mod := mh.Module
	for _, wtPath := range existingPaths {
		otherHash, err := HashLockfile(wtPath, mod.Lockfile)
		if err != nil || otherHash != mh.Hash {
			continue
		}

		modDir := filepath.Join(wtPath, mod.Dir)
		if info, err := os.Stat(modDir); err != nil || !info.IsDir() {
			continue
		}

		if err := CloneModule(wtPath, worktreePath, mod); err != nil {
			if !mod.CloneOnly {
				installErr := runInstall(worktreePath, mod)
				return InstallResult{Module: mod, Error: installErr}, true
			}
			return InstallResult{Module: mod, Error: fmt.Errorf("clone from %s: %w", wtPath, err)}, true
		}

		return InstallResult{Module: mod, Source: wtPath, Cloned: true}, true
	}
	return InstallResult{}, false
}

func runInstall(worktreePath string, mod Module) error {
	dir := worktreePath
	if mod.WorkDir != "" {
		dir = filepath.Join(worktreePath, mod.WorkDir)
	}

	cmd := exec.Command("sh", "-c", mod.InstallCmd) //nolint:gosec // install commands come from user config
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install %q in %s: %w\n%s\nTo fix: cd %s && %s",
			mod.Dir, dir, err, strings.TrimSpace(buf.String()), dir, mod.InstallCmd)
	}
	return nil
}
