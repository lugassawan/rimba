package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/trust"
	"github.com/spf13/cobra"
)

const (
	flagYes = "yes"

	hintYes = "approve committed shell commands without prompting (use in CI; see 'rimba trust')"
)

// ensureTrust checks that the user has approved the committed shell commands
// in cfg for repoRoot. Returns nil when already trusted, the user approves,
// or --yes / RIMBA_TRUST_YES is set. Returns an error with a remediation hint
// when the user declines or stdin is non-interactive (EOF → default no).
func ensureTrust(cmd *cobra.Command, repoRoot string, cfg *config.Config) error {
	if !trust.HasCommands(cfg) {
		return nil
	}
	h := trust.Hash(cfg)
	ok, err := trust.IsTrusted(repoRoot, h)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	if trustYesRequested(cmd) {
		return trust.Record(repoRoot, h)
	}
	if !promptTrust(cmd, cfg) {
		return errhint.WithFix(
			errors.New("committed shell commands require approval"),
			"review the commands above, then run 'rimba trust' (or pass --yes / set RIMBA_TRUST_YES=1 in CI)",
		)
	}
	return trust.Record(repoRoot, h)
}

// promptTrust displays the committed shell commands and asks the user to
// approve them. Returns true only for "y" or "yes". Default is no.
func promptTrust(cmd *cobra.Command, cfg *config.Config) bool {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "\nThis repo's .rimba/settings.toml will run shell commands that have not been approved:")
	cmds := trust.Commands(cfg)
	for _, c := range cmds {
		fmt.Fprintf(out, "  %s\n", c)
	}
	fmt.Fprint(out, "\nRun these commands? [y/N] ")

	reader := bufio.NewReader(cmd.InOrStdin())
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// trustYesRequested returns true when --yes is set or RIMBA_TRUST_YES is truthy.
// The env-var truthiness is defined by trust.TrustYesFromEnv.
func trustYesRequested(cmd *cobra.Command) bool {
	if yes, _ := cmd.Flags().GetBool(flagYes); yes {
		return true
	}
	return trust.TrustYesFromEnv()
}
