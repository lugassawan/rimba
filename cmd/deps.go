package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

type depsStatusJSONItem struct {
	Branch  string                `json:"branch"`
	Path    string                `json:"path"`
	Modules []deps.ModuleWithHash `json:"modules"`
	Error   string                `json:"error,omitempty"`
}

func init() {
	depsCmd.AddCommand(depsStatusCmd)
	depsCmd.AddCommand(depsInstallCmd)
	rootCmd.AddCommand(depsCmd)
}

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage worktree dependencies",
	Long:  "Detect, inspect, and install shared dependencies across worktrees.",
}

var depsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detected modules and lockfile hashes for all worktrees",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner()
		worktrees, err := listWorktreeInfos(r)
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

				modules, err := deps.ResolveModules(wt.Path, cfg.IsAutoDetectDeps(), configModules, existingPaths)
				if err != nil {
					item.Error = err.Error()
					item.Modules = make([]deps.ModuleWithHash, 0)
					items = append(items, item)
					continue
				}

				if len(modules) == 0 {
					item.Modules = make([]deps.ModuleWithHash, 0)
					items = append(items, item)
					continue
				}

				hashed, err := deps.HashModules(wt.Path, modules)
				if err != nil {
					item.Error = err.Error()
					item.Modules = make([]deps.ModuleWithHash, 0)
					items = append(items, item)
					continue
				}

				item.Modules = hashed
				items = append(items, item)
			}
			return output.WriteJSON(cmd.OutOrStdout(), version, "deps status", items)
		}

		out := cmd.OutOrStdout()

		for _, wt := range worktrees {
			modules, err := deps.ResolveModules(wt.Path, cfg.IsAutoDetectDeps(), configModules, existingPaths)
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
				fmt.Fprintf(out, "  %s [%s]\n", mh.Module.Dir, hash)
			}
		}

		return nil
	},
}

var depsInstallCmd = &cobra.Command{
	Use:   "install <task>",
	Short: "Install dependencies for a specific worktree",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeWorktreeTasks(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]
		cfg := config.FromContext(cmd.Context())

		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		worktrees, err := listWorktreeInfos(r)
		if err != nil {
			return err
		}

		existingPaths := make([]string, len(worktrees))
		for i, w := range worktrees {
			existingPaths[i] = w.Path
		}

		prefixes := resolver.AllPrefixes()
		wt, found := resolver.FindBranchForTask(task, worktrees, prefixes)
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
				return fmt.Errorf(errWorktreeNotFound, task)
			}
		}

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		modules, err := deps.ResolveModules(wt.Path, cfg.IsAutoDetectDeps(), configModules, existingPaths)
		if err != nil {
			return err
		}

		if len(modules) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No modules detected for %q\n", task)
			return nil
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		s.Start("Installing dependencies...")
		mgr := &deps.Manager{Runner: r}
		results := mgr.Install(wt.Path, modules, func(cur, total int, name string) {
			s.Update(fmt.Sprintf("Installing dependencies... (%s) [%d/%d]", name, cur, total))
		})
		s.Stop()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Dependencies for %q:\n", task)
		for _, res := range results {
			if res.Cloned {
				fmt.Fprintf(out, "  %s: cloned from %s\n", res.Module.Dir, filepath.Base(res.Source))
			} else if res.Error != nil {
				fmt.Fprintf(out, "  %s: %v\n", res.Module.Dir, res.Error)
			} else if res.Module.InstallCmd != "" && !res.Module.CloneOnly {
				fmt.Fprintf(out, "  %s: installed\n", res.Module.Dir)
			} else {
				fmt.Fprintf(out, "  %s: skipped\n", res.Module.Dir)
			}
		}

		return nil
	},
}
