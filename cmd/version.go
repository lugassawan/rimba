package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:         "version",
	Short:       "Print the version information",
	Annotations: map[string]string{"skipConfig": "true"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "rimba %s (commit: %s, built: %s)\n", version, commit, date)
	},
}

// Version returns the current version string.
func Version() string {
	return version
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
