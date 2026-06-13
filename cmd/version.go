package cmd

import (
	"fmt"
	"runtime"

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
	Example:     "  rimba version",
	Annotations: map[string]string{"skipConfig": "true"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), versionString())
	},
}

// Version returns the current version string.
func Version() string {
	return version
}

// versionString returns the multi-line version block printed by both
// `rimba version` and `rimba --version`.
func versionString() string {
	return fmt.Sprintf(
		"rimba %s\ncommit: %s\nbuilt:  %s\nos:     %s\narch:   %s\ngo:     %s\n",
		version, commit, date, runtime.GOOS, runtime.GOARCH, runtime.Version(),
	)
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}")
	// Pre-register --version without a -v shorthand; this prevents Cobra's
	// InitDefaultVersionFlag from auto-adding -v when the flag is absent.
	rootCmd.Flags().Bool("version", false, "version for rimba")
}
