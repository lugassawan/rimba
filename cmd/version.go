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
	Annotations: map[string]string{"skipConfig": "true"},
	Run: func(cmd *cobra.Command, args []string) {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "rimba %s\n", version)
		fmt.Fprintf(w, "commit: %s\n", commit)
		fmt.Fprintf(w, "built:  %s\n", date)
		fmt.Fprintf(w, "os:     %s\n", runtime.GOOS)
		fmt.Fprintf(w, "arch:   %s\n", runtime.GOARCH)
	},
}

// Version returns the current version string.
func Version() string {
	return version
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
