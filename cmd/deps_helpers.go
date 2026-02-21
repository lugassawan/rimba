package cmd

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/deps"
)

func printInstallResults(out io.Writer, results []deps.InstallResult) {
	var printed bool
	for _, r := range results {
		if !r.Cloned && r.Error == nil {
			continue
		}
		if !printed {
			fmt.Fprintf(out, "  Dependencies:\n")
			printed = true
		}
		if r.Cloned {
			fmt.Fprintf(out, "    %s: cloned from %s\n", r.Module.Dir, filepath.Base(r.Source))
		} else if r.Error != nil {
			fmt.Fprintf(out, "    %s: %v\n", r.Module.Dir, r.Error)
		}
	}
}

// printHookResultsList prints pre-computed hook results.
func printHookResultsList(out io.Writer, results []deps.HookResult) {
	if len(results) == 0 {
		return
	}
	fmt.Fprintf(out, "  Hooks:\n")
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(out, "    %s: %v\n", r.Command, r.Error)
		} else {
			fmt.Fprintf(out, "    %s: ok\n", r.Command)
		}
	}
}
