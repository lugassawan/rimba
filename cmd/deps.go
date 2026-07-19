package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const flagPath = "path"

// depsStatusModuleJSON adds the on-disk install state to a ModuleWithHash
// for `deps status --json` — computed relative to a specific worktree path,
// so it can't live on ModuleWithHash itself.
type depsStatusModuleJSON struct {
	deps.ModuleWithHash
	InstallState string `json:"install_state"`
}

type depsStatusJSONItem struct {
	Branch  string                 `json:"branch"`
	Path    string                 `json:"path"`
	Modules []depsStatusModuleJSON `json:"modules"`
	Error   string                 `json:"error,omitempty"`
}

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage worktree dependencies",
	Long:  "Detect, inspect, and install shared dependencies across worktrees.",
}

var depsStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show detected modules and lockfile hashes for all worktrees",
	Example: "  rimba deps status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner(cmd.Context())
		worktrees, err := listWorktreeInfos(cmd.Context(), r)
		if err != nil {
			return err
		}

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		existingPaths := make([]string, len(worktrees))
		for i, w := range worktrees {
			existingPaths[i] = w.Path
		}

		if isJSON(cmd) {
			items := make([]depsStatusJSONItem, 0, len(worktrees))
			for _, wt := range worktrees {
				item := depsStatusJSONItem{
					Branch: wt.Branch,
					Path:   wt.Path,
				}

				modules, err := deps.ResolveModules(wt.Path, wt.Service, cfg.IsAutoDetectDeps(), configModules, existingPaths)
				if err != nil {
					item.Error = err.Error()
					item.Modules = make([]depsStatusModuleJSON, 0)
					items = append(items, item)
					continue
				}

				if len(modules) == 0 {
					item.Modules = make([]depsStatusModuleJSON, 0)
					items = append(items, item)
					continue
				}

				hashed, err := deps.HashModules(wt.Path, modules)
				if err != nil {
					item.Error = err.Error()
					item.Modules = make([]depsStatusModuleJSON, 0)
					items = append(items, item)
					continue
				}

				jsonModules := make([]depsStatusModuleJSON, len(hashed))
				for i, mh := range hashed {
					jsonModules[i] = depsStatusModuleJSON{ModuleWithHash: mh, InstallState: mh.Module.InstallState(wt.Path)}
				}
				item.Modules = jsonModules
				items = append(items, item)
			}
			return output.WriteJSON(cmd.OutOrStdout(), version, "deps status", items)
		}

		out := cmd.OutOrStdout()

		for _, wt := range worktrees {
			modules, err := deps.ResolveModules(wt.Path, wt.Service, cfg.IsAutoDetectDeps(), configModules, existingPaths)
			if err != nil {
				fmt.Fprintf(out, "%s (%s)\n  error: %v\n", wt.Branch, wt.Path, err)
				continue
			}

			fmt.Fprintf(out, "%s (%s)\n", wt.Branch, wt.Path)

			if len(modules) == 0 {
				fmt.Fprintf(out, "  (no modules detected)\n")
				continue
			}

			hashed, err := deps.HashModules(wt.Path, modules)
			if err != nil {
				fmt.Fprintf(out, "  error hashing: %v\n", err)
				continue
			}

			for _, mh := range hashed {
				hash := mh.Hash
				if len(hash) > 12 {
					hash = hash[:12]
				}
				if hash == "" {
					hash = "(no lockfile)"
				}
				fmt.Fprintf(out, "  %s [%s] %s\n", mh.Module.Dir, hash, mh.Module.InstallState(wt.Path))
			}
		}

		return nil
	},
}

var depsInstallCmd = &cobra.Command{
	Use:     "install <task>",
	Short:   "Install dependencies for a specific worktree",
	Example: "  rimba deps install auth",
	Args:    cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		cfg := config.FromContext(cmd.Context())

		r := newRunner(cmd.Context())

		repoRoot, err := git.MainRepoRoot(cmd.Context(), r)
		if err != nil {
			return err
		}

		worktrees, err := listWorktreeInfos(cmd.Context(), r)
		if err != nil {
			return err
		}

		existingPaths := make([]string, len(worktrees))
		for i, w := range worktrees {
			existingPaths[i] = w.Path
		}

		prefixes := cfg.PrefixSet().Strip()
		svc, resolvedTask := operations.ResolveTaskInput(task, repoRoot, cfg.PrefixSet())
		wt, found := resolver.FindBranchForTask(svc, resolvedTask, worktrees, prefixes)
		if !found {
			// Also try resolving as a worktree path
			wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
			for _, w := range worktrees {
				if w.Path == filepath.Join(wtDir, task) {
					wt = w
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf(operations.ErrWorktreeNotFoundFmt, task)
			}
		}

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		modules, err := deps.ResolveModules(wt.Path, wt.Service, cfg.IsAutoDetectDeps(), configModules, existingPaths)
		if err != nil {
			return err
		}

		if len(modules) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No modules detected for %q\n", task)
			return nil
		}

		if path, _ := cmd.Flags().GetString(flagPath); path != "" {
			modules, err = filterModulesByPath(modules, path)
			if err != nil {
				return err
			}
		}

		if err := ensureTrust(cmd, repoRoot, cfg); err != nil {
			return err
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		s.Start("Installing dependencies...")
		mgr := &deps.Manager{Runner: r, Concurrency: cfg.DepsConcurrency()}
		results := mgr.Install(cmd.Context(), wt.Path, modules, nil, func(msg string) {
			s.Update("Installing dependencies... " + msg)
		})
		s.Stop()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Dependencies for %q:\n", task)
		for _, res := range results {
			fmt.Fprintf(out, "  %s\n", formatDepsInstallLine(res))
		}

		return nil
	},
}

func init() {
	depsCmd.AddCommand(depsStatusCmd)
	depsCmd.AddCommand(depsInstallCmd)
	depsInstallCmd.Flags().String(flagPath, "", "install only the module at this dir (e.g. standalone-svc-a/node_modules)")
	rootCmd.AddCommand(depsCmd)
}

// formatDepsInstallLine renders one module's install outcome for `rimba deps install`.
// Deferred can't actually occur here today (this command's Manager leaves
// SkipDeferred false), but is handled for defensive completeness.
func formatDepsInstallLine(res deps.InstallResult) string {
	switch {
	case res.Deferred:
		return res.Module.Dir + ": deferred"
	case res.Cloned:
		return fmt.Sprintf("%s: cloned from %s", res.Module.Dir, filepath.Base(res.Source))
	case res.Error != nil:
		return fmt.Sprintf("%s: %v", res.Module.Dir, res.Error)
	case !res.Ran:
		return res.Module.Dir + ": skipped (cancelled)"
	case res.Module.InstallCmd != "" && !res.Module.CloneOnly:
		return res.Module.Dir + ": installed"
	default:
		return res.Module.Dir + ": skipped"
	}
}

// filterModulesByPath returns the single module whose Dir equals dir, or an
// error listing the available dirs if none match.
func filterModulesByPath(modules []deps.Module, dir string) ([]deps.Module, error) {
	for _, m := range modules {
		if m.Dir == dir {
			return []deps.Module{m}, nil
		}
	}
	available := make([]string, len(modules))
	for i, m := range modules {
		available[i] = m.Dir
	}
	return nil, fmt.Errorf("no module with dir %q; available: %s", dir, strings.Join(available, ", "))
}
