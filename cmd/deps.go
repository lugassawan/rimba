package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

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

		r := &git.ExecRunner{}
		worktrees, err := listWorktreeInfos(r)
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()

		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		existingPaths := make([]string, len(worktrees))
		for i, w := range worktrees {
			existingPaths[i] = w.Path
		}

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

		r := &git.ExecRunner{}

		repoRoot, err := git.RepoRoot(r)
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

		mgr := &deps.Manager{Runner: r}
		results := mgr.Install(wt.Path, modules)

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
