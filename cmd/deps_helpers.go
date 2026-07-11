package cmd

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/deps"
	"github.com/lugassawan/rimba/internal/output"
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

// buildDepResults maps dependency install results to their JSON representation.
func buildDepResults(results []deps.InstallResult) []output.DepResultJSON {
	out := make([]output.DepResultJSON, 0, len(results))
	for _, r := range results {
		out = append(out, output.DepResultJSON{
			Module: r.Module.Dir,
			Source: r.Source,
			Cloned: r.Cloned,
			Error:  errStr(r.Error),
		})
	}
	return out
}

// buildHookResults maps post-create hook results to their JSON representation.
func buildHookResults(results []deps.HookResult) []output.HookResultJSON {
	out := make([]output.HookResultJSON, 0, len(results))
	for _, r := range results {
		out = append(out, output.HookResultJSON{
			Command: r.Command,
			Error:   errStr(r.Error),
		})
	}
	return out
}
