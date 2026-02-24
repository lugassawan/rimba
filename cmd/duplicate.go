package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/hint"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/spf13/cobra"
)

const (
	flagAs = "as"

	hintAs = "Use a custom name instead of auto-suffix (-1, -2, etc.)"

	maxDuplicateSuffix = 1000
)

func init() {
	duplicateCmd.Flags().String(flagAs, "", "Custom name for the duplicate worktree")
	duplicateCmd.Flags().Bool(flagSkipDeps, false, "Skip dependency detection and installation")
	duplicateCmd.Flags().Bool(flagSkipHooks, false, "Skip post-create hooks")
	rootCmd.AddCommand(duplicateCmd)
}

var duplicateCmd = &cobra.Command{
	Use:   "duplicate <task>",
	Short: "Create a new worktree from an existing worktree",
	Long:  "Creates a new worktree branched from an existing worktree's branch, inheriting its prefix. Auto-suffixes with -1, -2, etc. unless --as is provided.",
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

		wt, err := findWorktree(r, task)
		if err != nil {
			return err
		}

		prefixes := resolver.AllPrefixes()

		if wt.Branch == cfg.DefaultSource {
			return fmt.Errorf("cannot duplicate the default branch %q; use 'rimba add' instead", cfg.DefaultSource)
		}

		// Extract prefix from source branch
		_, matchedPrefix := resolver.TaskFromBranch(wt.Branch, prefixes)
		if matchedPrefix == "" {
			matchedPrefix, _ = resolver.PrefixString(resolver.DefaultPrefixType)
		}

		// Determine new task name
		asFlag, _ := cmd.Flags().GetString(flagAs)
		var newTask string
		if asFlag != "" {
			newTask = asFlag
		} else {
			// Auto-suffix: try task-1, task-2, etc.
			for i := 1; i <= maxDuplicateSuffix; i++ {
				candidate := fmt.Sprintf("%s-%d", task, i)
				candidateBranch := resolver.BranchName(matchedPrefix, candidate)
				if !git.BranchExists(r, candidateBranch) {
					newTask = candidate
					break
				}
			}
			if newTask == "" {
				return fmt.Errorf("could not find available suffix for %q (tried 1-%d); use --as to specify a name", task, maxDuplicateSuffix)
			}
		}

		newBranch := resolver.BranchName(matchedPrefix, newTask)
		wtDir := filepath.Join(repoRoot, cfg.WorktreeDir)
		wtPath := resolver.WorktreePath(wtDir, newBranch)

		// Validate
		if git.BranchExists(r, newBranch) {
			return fmt.Errorf("branch %q already exists", newBranch)
		}
		if _, err := os.Stat(wtPath); err == nil {
			return fmt.Errorf("worktree path already exists: %s", wtPath)
		}

		hint.New(cmd, hintPainter(cmd)).
			Add(flagSkipDeps, hintSkipDeps).
			Add(flagSkipHooks, hintSkipHooks).
			Add(flagAs, hintAs).
			Show()

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		// Create worktree from source branch
		s.Start("Creating worktree...")
		if err := git.AddWorktree(r, wtPath, newBranch, wt.Branch); err != nil {
			return err
		}

		// Post-create setup: copy files, deps, hooks
		skipDeps, _ := cmd.Flags().GetBool(flagSkipDeps)
		skipHooks, _ := cmd.Flags().GetBool(flagSkipHooks)
		var configModules []config.ModuleConfig
		if cfg.Deps != nil {
			configModules = cfg.Deps.Modules
		}

		pcResult, err := operations.PostCreateSetup(r, operations.PostCreateParams{
			RepoRoot:      repoRoot,
			WtPath:        wtPath,
			Task:          newTask,
			CopyFiles:     cfg.CopyFiles,
			SkipDeps:      skipDeps,
			AutoDetect:    cfg.IsAutoDetectDeps(),
			ConfigModules: configModules,
			SkipHooks:     skipHooks,
			PostCreate:    cfg.PostCreate,
			SourcePath:    wt.Path,
		}, func(msg string) { s.Update(msg) })
		if err != nil {
			return err
		}

		s.Stop()

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Duplicated worktree %q as %q\n", task, newTask)
		fmt.Fprintf(out, "  Branch: %s\n", newBranch)
		fmt.Fprintf(out, "  Path:   %s\n", wtPath)
		if len(pcResult.Copied) > 0 {
			fmt.Fprintf(out, "  Copied: %v\n", pcResult.Copied)
		}
		if len(pcResult.Skipped) > 0 {
			fmt.Fprintf(out, "  Skipped (not found): %v\n", pcResult.Skipped)
		}

		printInstallResults(out, pcResult.DepsResults)
		printHookResultsList(out, pcResult.HookResults)

		return nil
	},
}
