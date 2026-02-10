package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

func printBanner(cmd *cobra.Command) {
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	lines := []string{
		`     _       _`,
		` _ _(_)_ __ | |__  __ _`,
		"| '_| | '  \\| '_ \\/ _` |",
		`|_| |_|_|_|_|_.__/\__,_|`,
	}

	versionStr := "v" + Version()

	for i, line := range lines {
		colored := p.Paint(line, termcolor.Green, termcolor.Bold)
		if i == 1 {
			fmt.Fprintf(cmd.OutOrStdout(), "%s          %s\n", colored, p.Paint(versionStr, termcolor.Gray))
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), colored)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())
}
