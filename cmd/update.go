package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/spinner"
	"github.com/lugassawan/rimba/internal/updater"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update rimba to the latest version",
	Example: `  rimba update
  rimba update --force`,
	Long: `Check for the latest release on GitHub and update the binary in place.

If the current binary cannot be replaced due to file permissions, rimba installs the
new version to ~/.local/bin instead and prints the path.

After a successful update, rimba prints a one-line tip if agent files are installed at
user level (run rimba init -g to refresh) or in this repo (run rimba init --agents to
refresh). Set RIMBA_QUIET=1 to suppress the tip.`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		if updater.IsDevVersion(version) && !force {
			fmt.Fprintln(cmd.OutOrStdout(), "You are running a development build of rimba.")
			fmt.Fprintln(cmd.OutOrStdout(), "Use --force to update anyway.")
			return nil
		}

		s := spinner.New(spinnerOpts(cmd))
		defer s.Stop()

		r := updater.NewRunner(version)
		r.Out = cmd.OutOrStdout()
		r.Spinner = s
		r.OnSuccess = func() {
			home, repoRoot := resolvePostUpdateTipPaths()
			printAgentRefreshTips(cmd, home, repoRoot)
		}

		return r.Run(cmd.Context())
	},
}

func init() {
	updateCmd.Flags().Bool("force", false, "update even if running a development build")
	rootCmd.AddCommand(updateCmd)
}
