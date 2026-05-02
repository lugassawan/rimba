package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/operations"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:         "completion <bash|zsh|fish|powershell>",
	Short:       "Generate shell completion scripts",
	ValidArgs:   []string{"bash", "zsh", "fish", "powershell"},
	Args:        cobra.ExactArgs(1),
	Annotations: map[string]string{"skipConfig": "true"},
	Long: `Generate shell completion scripts for rimba.

Bash:
  # Load for the current session:
  source <(rimba completion bash)

  # Install permanently (Linux):
  rimba completion bash > /etc/bash_completion.d/rimba

  # Install permanently (macOS with Homebrew bash-completion@2):
  rimba completion bash > "$(brew --prefix)/etc/bash_completion.d/rimba"

Zsh:
  # Load for the current session:
  source <(rimba completion zsh)

  # Install permanently:
  rimba completion zsh > "${fpath[1]}/_rimba"

Fish:
  rimba completion fish > ~/.config/fish/completions/rimba.fish

PowerShell:
  rimba completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session:
  rimba completion powershell > rimba.ps1
  # then source rimba.ps1 from your PowerShell profile`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(out, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(out)
		case "fish":
			return cmd.Root().GenFishCompletion(out, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(out)
		default:
			return fmt.Errorf("unsupported shell %q: choose bash, zsh, fish, or powershell", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// completeWorktreeTasks returns task names for shell completion.
func completeWorktreeTasks(_ *cobra.Command, toComplete string) []string {
	r := newRunner()
	entries, err := git.ListWorktrees(r)
	if err != nil {
		return nil
	}

	prefixes := resolver.AllPrefixes()
	var tasks []string
	for _, e := range entries {
		if e.Bare || e.Branch == "" {
			continue
		}
		task, _ := resolver.PureTaskFromBranch(e.Branch, prefixes)
		if strings.HasPrefix(task, toComplete) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// completeOpenShortcuts returns shortcut names from [open] config for shell completion.
func completeOpenShortcuts(cmd *cobra.Command, toComplete string) []string {
	cfg := config.FromContext(cmd.Context())
	if cfg == nil || cfg.Open == nil {
		return nil
	}

	var names []string
	for k := range cfg.Open {
		if strings.HasPrefix(k, toComplete) {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}

// completeArchivedTasks returns task names from archived branches (branches not in any active worktree).
func completeArchivedTasks(_ *cobra.Command, toComplete string) []string {
	r := newRunner()

	mainBranch, _ := resolveMainBranch(r)

	archived, err := operations.ListArchivedBranches(r, mainBranch)
	if err != nil {
		return nil
	}

	prefixes := resolver.AllPrefixes()
	var tasks []string
	for _, b := range archived {
		task, _ := resolver.PureTaskFromBranch(b, prefixes)
		if strings.HasPrefix(task, toComplete) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// completeBranchNames returns branch names for shell completion.
func completeBranchNames(_ *cobra.Command, toComplete string) []string {
	r := newRunner()
	out, err := r.Run("branch", "--format=%(refname:short)")
	if err != nil {
		return nil
	}

	var branches []string
	for b := range strings.SplitSeq(out, "\n") {
		b = strings.TrimSpace(b)
		if b != "" && strings.HasPrefix(b, toComplete) {
			branches = append(branches, b)
		}
	}
	return branches
}
