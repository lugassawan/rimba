package cmd

import (
	"errors"

	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/spf13/cobra"
)

type initMode struct {
	global    bool
	agents    bool
	local     bool
	uninstall bool
}

func initModeFromFlags(cmd *cobra.Command) initMode {
	g, _ := cmd.Flags().GetBool(flagGlobal)
	a, _ := cmd.Flags().GetBool(flagAgents)
	l, _ := cmd.Flags().GetBool(flagLocal)
	u, _ := cmd.Flags().GetBool(flagUninstall)
	return initMode{global: g, agents: a, local: l, uninstall: u}
}

func (m initMode) validate() error {
	if m.local && !m.agents {
		return errhint.WithFix(
			errors.New("--local requires --agents"),
			"run: rimba init --agents --local",
		)
	}
	if m.uninstall && !m.global && !m.agents {
		return errhint.WithFix(
			errors.New("--uninstall requires -g or --agents"),
			"run: rimba init -g --uninstall  OR  rimba init --agents --uninstall",
		)
	}
	return nil
}
