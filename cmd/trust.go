package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/spf13/cobra"
)

const flagShow = "show"

var trustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Approve committed shell commands for this repo",
	Long: `Review and approve the shell commands in .rimba/settings.toml.

rimba will not run committed post_create, post_rename, or deps.modules[].install
shell commands until you approve them. Approval is stored locally in
.rimba/trust.local.toml (gitignored) and is keyed by a hash of the command set.
Changing the commands re-arms the consent gate.

Use --show to inspect the current command set and approval status without prompting.
Use --yes to approve without prompting (e.g. in CI, combined with RIMBA_TRUST_YES=1).`,
	Example: `  rimba trust           # review and approve
  rimba trust --show    # inspect without approving
  rimba trust --yes     # approve without prompting`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())

		r := newRunner(cmd.Context())
		repoRoot, err := git.MainRepoRoot(cmd.Context(), r)
		if err != nil {
			return err
		}

		show, _ := cmd.Flags().GetBool(flagShow)
		if show {
			return runTrustShow(cmd, repoRoot, cfg)
		}
		return runTrustApprove(cmd, repoRoot, cfg)
	},
}

// buildTrustCmd constructs a testable trust command with injected deps.
// Used by trust_test.go to avoid needing a real git repo.
func buildTrustCmd(cfg *config.Config, repoRoot string) *cobra.Command {
	c := &cobra.Command{
		Use:  "trust",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			show, _ := cmd.Flags().GetBool(flagShow)
			if show {
				return runTrustShow(cmd, repoRoot, cfg)
			}
			return runTrustApprove(cmd, repoRoot, cfg)
		},
	}
	c.Flags().Bool(flagShow, false, "")
	c.Flags().Bool(flagYes, false, "")
	return c
}

func runTrustApprove(cmd *cobra.Command, repoRoot string, cfg *config.Config) error {
	if !trust.HasCommands(cfg) {
		fmt.Fprintln(cmd.OutOrStdout(), "No shell commands in .rimba/settings.toml — nothing to approve.")
		return nil
	}

	h := trust.Hash(cfg)
	ok, err := trust.IsTrusted(repoRoot, h)
	if err != nil {
		return err
	}
	if ok {
		fmt.Fprintln(cmd.OutOrStdout(), "Already trusted — no changes needed.")
		return nil
	}

	if !trustYesRequested(cmd) && !promptTrust(cmd, cfg) {
		fmt.Fprintln(cmd.OutOrStdout(), "Approval declined.")
		return nil
	}
	if err := trust.Record(repoRoot, h); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Approved. rimba will run these commands without prompting.")
	return nil
}

func runTrustShow(cmd *cobra.Command, repoRoot string, cfg *config.Config) error {
	out := cmd.OutOrStdout()

	if !trust.HasCommands(cfg) {
		fmt.Fprintln(out, "No shell commands configured in .rimba/settings.toml.")
		return nil
	}

	fmt.Fprintln(out, "Configured shell commands:")
	for _, c := range trust.Commands(cfg) {
		fmt.Fprintf(out, "  %s\n", c)
	}

	h := trust.Hash(cfg)
	fmt.Fprintf(out, "\nHash: %s\n", h)

	ok, err := trust.IsTrusted(repoRoot, h)
	if err != nil {
		return err
	}
	if ok {
		fmt.Fprintln(out, "Status: trusted")
	} else {
		fmt.Fprintln(out, "Status: not trusted — run 'rimba trust' to approve")
	}
	return nil
}

func init() {
	trustCmd.Flags().Bool(flagShow, false, "display commands and approval status without prompting")
	trustCmd.Flags().Bool(flagYes, false, hintYes)
	rootCmd.AddCommand(trustCmd)
}
