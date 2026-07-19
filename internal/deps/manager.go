package deps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/debug"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/observability"
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

	// Reflink is true when Cloned was a genuine CoW reflink/clonefile (per
	// cowEligible), false when Cloned was a byte-copy (CloneOnly modules
	// forced through on a non-CoW filesystem). Meaningless when !Cloned.
	Reflink bool

	Error error

	// Ran is true only if this module's install goroutine actually executed,
	// distinguishing a cancelled dispatch from a real no-op.
	Ran bool
}

// Install clones or installs deps for each module.
// Pass existingEntries to skip an extra git.ListWorktrees call; nil fetches its own.
func (m *Manager) Install(ctx context.Context, worktreePath string, modules []Module, existingEntries []git.WorktreeEntry, onProgress progress.Func) []InstallResult {
	return m.install(ctx, worktreePath, "", modules, existingEntries, onProgress)
}

// InstallPreferSource is like Install but tries sourceWT first when cloning.
func (m *Manager) InstallPreferSource(ctx context.Context, worktreePath, sourceWT string, modules []Module, existingEntries []git.WorktreeEntry, onProgress progress.Func) []InstallResult {
	return m.install(ctx, worktreePath, sourceWT, modules, existingEntries, onProgress)
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

func (m *Manager) install(ctx context.Context, worktreePath, sourceWT string, modules []Module, existingEntries []git.WorktreeEntry, onProgress progress.Func) []InstallResult {
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
		entries, err = git.ListWorktrees(ctx, m.Runner)
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

	// No per-item timeout here — dependency installation is long-running by design.
	results = parallel.Collect(ctx, total, concurrency, func(ctx context.Context, i int) InstallResult {
		res := m.installModule(ctx, worktreePath, hashed[i], existingPaths)
		res.Ran = true
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

// installModule wraps installModuleInner with a module-level observability
// span, recording whether the module was cloned via a true reflink, cloned
// via a byte-copy, or freshly installed.
func (m *Manager) installModule(ctx context.Context, worktreePath string, mh ModuleWithHash, existingPaths []string) InstallResult {
	rec := observability.FromContext(ctx)
	stop := rec.StartModuleSpan(mh.Module.Dir)
	result := m.installModuleInner(ctx, worktreePath, mh, existingPaths)
	stop(moduleSpanDetail(result))
	return result
}

// moduleSpanDetail maps an InstallResult to the observability detail label
// distinguishing a true reflink clone (fast) from a byte-copy clone (the
// pessimization Stage 1 exists to avoid for install-capable modules) from a
// fresh install.
func moduleSpanDetail(result InstallResult) string {
	if !result.Cloned {
		return observability.DetailInstalled
	}
	if result.Reflink {
		return observability.DetailClonedReflink
	}
	return observability.DetailClonedCopy
}

func (m *Manager) installModuleInner(ctx context.Context, worktreePath string, mh ModuleWithHash, existingPaths []string) InstallResult {
	mod := mh.Module

	if mh.Hash == "" {
		return InstallResult{Module: mod}
	}

	if result, ok := tryCloneFromExisting(ctx, worktreePath, mh, existingPaths); ok {
		return result
	}

	if mod.CloneOnly {
		return InstallResult{Module: mod}
	}

	if mod.InstallCmd != "" {
		err := runInstall(ctx, worktreePath, mod)
		return InstallResult{Module: mod, Error: err}
	}

	return InstallResult{Module: mod}
}

// tryCloneFromExisting attempts to clone the module from an existing worktree
// with a matching lockfile. For install-capable modules (mod.InstallCmd set),
// a clone is only attempted when cowEligible confirms the dst filesystem
// truly honors a reflink/clonefile — otherwise it's skipped in favor of the
// install path, which is dramatically cheaper than a byte-copy of a large
// dependency tree (see cowEligible's doc comment). CloneOnly modules (no
// install fallback to fall back to) and modules with no InstallCmd at all
// always attempt the clone, matching pre-existing behavior.
//
// Recursive install-capable modules (pnpm/yarn/npm node_modules) are a
// special case: cowEligible only measures whether ONE file reflinks, but a
// Recursive clone walks and clones every nested node_modules dir in a
// monorepo — an unbounded cost that scales with workspace count. Confirmed
// empirically: a genuine reflink clone of a 100k+-entry node_modules still
// took 100+s (syscall-per-entry overhead, not a byte-copy in disguise),
// while the real install stayed at 2-5s regardless (the package manager's
// own store materializes it). So Recursive install-capable modules always
// install — the probe is never even consulted for them.
func tryCloneFromExisting(ctx context.Context, worktreePath string, mh ModuleWithHash, existingPaths []string) (InstallResult, bool) {
	mod := mh.Module

	if mod.Recursive && mod.InstallCmd != "" && !mod.CloneOnly {
		return InstallResult{}, false
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

		reflink := cowEligible(ctx, modDir, worktreePath)
		if mod.InstallCmd != "" && !mod.CloneOnly && !reflink {
			continue // prefer install over a byte-copy in disguise
		}

		return cloneAndPost(ctx, worktreePath, wtPath, mod, reflink), true
	}
	return InstallResult{}, false
}

// cloneAndPost clones the module from srcWT to dstWT and runs the PostClone hook.
// reflink records whether cowEligible confirmed a true CoW clone (for observability).
func cloneAndPost(ctx context.Context, dstWT, srcWT string, mod Module, reflink bool) InstallResult {
	if err := CloneModule(ctx, srcWT, dstWT, mod); err != nil {
		if !mod.CloneOnly {
			installErr := runInstall(ctx, dstWT, mod)
			return InstallResult{Module: mod, Error: installErr}
		}
		return InstallResult{Module: mod, Error: fmt.Errorf("clone from %s: %w", srcWT, err)}
	}

	if mod.PostClone != nil {
		if err := mod.PostClone(srcWT, dstWT, mod); err != nil {
			_ = os.RemoveAll(filepath.Join(dstWT, mod.Dir))
			return InstallResult{Module: mod, Error: fmt.Errorf("post-clone %s: %w", mod.Dir, err)}
		}
	}

	return InstallResult{Module: mod, Source: srcWT, Cloned: true, Reflink: reflink}
}

func runInstall(ctx context.Context, worktreePath string, mod Module) error {
	dir := worktreePath
	if mod.WorkDir != "" {
		dir = filepath.Join(worktreePath, mod.WorkDir)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", mod.InstallCmd) //nolint:gosec // install commands come from user config
	cmd.Dir = dir
	configureProcessGroup(cmd)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	start := time.Now()
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	observability.FromContext(ctx).LogSubprocess(observability.CategoryExec, dir, []string{mod.InstallCmd}, exitCode, time.Since(start), buf.String(), err != nil)

	if err != nil {
		wrapped := fmt.Errorf("install %q in %s: %w\n%s",
			mod.Dir, dir, err, strings.TrimSpace(buf.String()))
		return errhint.WithFix(wrapped, fmt.Sprintf("cd %s && %s", dir, mod.InstallCmd))
	}
	return nil
}
