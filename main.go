package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lugassawan/rimba/cmd"
	"github.com/lugassawan/rimba/internal/output"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var silent *output.SilentError
		if errors.As(err, &silent) {
			os.Exit(silent.ExitCode)
		}

		if cmd.IsJSONMode() {
			_ = output.WriteJSONError(os.Stdout, cmd.Version(), cmd.CommandName(), err.Error(), output.ErrGeneral)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
