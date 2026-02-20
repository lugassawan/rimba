package deps

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
)

// ProgressFunc is called before each item is processed to report progress.
// current is 1-based, total is the count of items, name identifies the item.
type ProgressFunc func(current, total int, name string)

// Manager orchestrates dependency installation for worktrees.
type Manager struct {
	Runner git.Runner
}

// InstallResult holds the outcome of installing a single module.
type InstallResult struct {
	Module Module
	Source string // source worktree path if cloned
	Cloned bool
	Error  error
}

// Install detects matching worktrees and clones or installs dependencies.
func (m *Manager) Install(worktreePath string, modules []Module, onProgress ProgressFunc) []InstallResult {
	results := make([]InstallResult, 0, len(modules))

	hashed, err := HashModules(worktreePath, modules)
	if err != nil {
		for _, mod := range modules {
			results = append(results, InstallResult{Module: mod, Error: err})
		}
		return results
	}

	entries, err := git.ListWorktrees(m.Runner)
	if err != nil {
		for _, mod := range modules {
			results = append(results, InstallResult{Module: mod, Error: err})
		}
		return results
	}

	var existingPaths []string
	for _, e := range entries {
		if e.Path != worktreePath {
			existingPaths = append(existingPaths, e.Path)
		}
	}

	for i, mh := range hashed {
		if onProgress != nil {
			onProgress(i+1, len(hashed), mh.Module.Dir)
		}
		result := m.installModule(worktreePath, mh, existingPaths)
		results = append(results, result)
	}

	return results
}

// InstallPreferSource is like Install but tries the given source worktree first.
// Used by `duplicate` to prefer cloning from the worktree being duplicated.
func (m *Manager) InstallPreferSource(worktreePath, sourceWT string, modules []Module, onProgress ProgressFunc) []InstallResult {
	results := make([]InstallResult, 0, len(modules))

	hashed, err := HashModules(worktreePath, modules)
	if err != nil {
		for _, mod := range modules {
			results = append(results, InstallResult{Module: mod, Error: err})
		}
		return results
	}

	entries, err := git.ListWorktrees(m.Runner)
	if err != nil {
		for _, mod := range modules {
			results = append(results, InstallResult{Module: mod, Error: err})
		}
		return results
	}

	var existingPaths []string
	existingPaths = append(existingPaths, sourceWT)
	for _, e := range entries {
		if e.Path != worktreePath && e.Path != sourceWT {
			existingPaths = append(existingPaths, e.Path)
		}
	}

	for i, mh := range hashed {
		if onProgress != nil {
			onProgress(i+1, len(hashed), mh.Module.Dir)
		}
		result := m.installModule(worktreePath, mh, existingPaths)
		results = append(results, result)
	}

	return results
}

func (m *Manager) installModule(worktreePath string, mh ModuleWithHash, existingPaths []string) InstallResult {
	mod := mh.Module

	if mh.Hash == "" {
		return InstallResult{Module: mod}
	}

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
				return InstallResult{Module: mod, Error: installErr}
			}
			return InstallResult{Module: mod, Error: fmt.Errorf("clone from %s: %w", wtPath, err)}
		}

		return InstallResult{Module: mod, Source: wtPath, Cloned: true}
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

// ResolveModules detects and merges modules, filtering clone-only ones.
func ResolveModules(worktreePath string, autoDetect bool, configModules []config.ModuleConfig, existingWTPaths []string) ([]Module, error) {
	var modules []Module

	if autoDetect {
		detected, err := DetectModules(worktreePath)
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
