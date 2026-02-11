package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/fleet"
	"github.com/lugassawan/rimba/internal/fleet/agent"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

const (
	flagManifest = "manifest"
	flagAgent    = "agent"
)

func init() {
	fleetSpawnCmd.Flags().String(flagManifest, "", "Path to fleet.toml manifest")
	fleetSpawnCmd.Flags().String(flagAgent, "", "Agent to use (claude, cursor, codex, generic)")

	fleetCmd.AddCommand(fleetSpawnCmd)
	fleetCmd.AddCommand(fleetStatusCmd)
	fleetCmd.AddCommand(fleetStopCmd)
	fleetCmd.AddCommand(fleetLogsCmd)

	rootCmd.AddCommand(fleetCmd)
}

var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "AI agent orchestration",
	Long:  "Spawn and manage AI agents across worktrees.",
}

var fleetSpawnCmd = &cobra.Command{
	Use:   "spawn [tasks...]",
	Short: "Spawn AI agents in worktrees",
	Long: `Creates worktrees and assigns AI agents to work on tasks.

Examples:
  rimba fleet spawn --agent claude auth-refactor fix-memory-leak
  rimba fleet spawn --manifest fleet.toml
  rimba fleet spawn --agent codex add-docs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		r := newRunner()

		manifestPath, _ := cmd.Flags().GetString(flagManifest)
		agentName, _ := cmd.Flags().GetString(flagAgent)

		if agentName == "" {
			agentName = cfg.FleetDefaultAgent()
		}

		var tasks []fleet.TaskSpec

		if manifestPath != "" {
			m, err := fleet.LoadManifest(manifestPath)
			if err != nil {
				return err
			}
			tasks = m.Tasks
		} else if len(args) > 0 {
			for _, name := range args {
				tasks = append(tasks, fleet.TaskSpec{
					Name:  name,
					Agent: agentName,
				})
			}
		} else {
			return fmt.Errorf("provide task names or --manifest")
		}

		mgr, err := fleet.NewManager(r, cfg, makeCommandFactory())
		if err != nil {
			return err
		}

		if err := mgr.EnsureDirs(); err != nil {
			return err
		}

		// Auto-scaffold fleet.toml if it doesn't exist and we're spawning inline
		if manifestPath == "" {
			repoRoot, err := git.RepoRoot(r)
			if err != nil {
				return err
			}
			fleetToml := filepath.Join(repoRoot, "fleet.toml")
			if _, err := os.Stat(fleetToml); os.IsNotExist(err) {
				m := &fleet.Manifest{Tasks: tasks}
				if err := fleet.SaveManifest(fleetToml, m); err != nil {
					return fmt.Errorf("auto-scaffold fleet.toml: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created fleet.toml\n")
			}
		}

		results := mgr.Spawn(tasks, agentName)

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		tbl := termcolor.NewTable(2)
		tbl.AddRow(
			p.Paint("TASK", termcolor.Bold),
			p.Paint("AGENT", termcolor.Bold),
			p.Paint("STATUS", termcolor.Bold),
		)

		for _, r := range results {
			status := p.Paint("spawned", termcolor.Green)
			if r.Error != nil {
				status = p.Paint(r.Error.Error(), termcolor.Red)
			}
			tbl.AddRow(r.Task.Name, agentName, status)
		}
		tbl.Render(cmd.OutOrStdout())
		return nil
	},
}

var fleetStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show fleet task status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		r := newRunner()

		mgr, err := fleet.NewManager(r, cfg, nil)
		if err != nil {
			return err
		}

		states, err := mgr.Status()
		if err != nil {
			return err
		}

		if len(states) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No fleet tasks found.")
			return nil
		}

		noColor, _ := cmd.Flags().GetBool(flagNoColor)
		p := termcolor.NewPainter(noColor)

		tbl := termcolor.NewTable(2)
		tbl.AddRow(
			p.Paint("TASK", termcolor.Bold),
			p.Paint("AGENT", termcolor.Bold),
			p.Paint("STATUS", termcolor.Bold),
			p.Paint("PID", termcolor.Bold),
			p.Paint("BRANCH", termcolor.Bold),
		)

		for _, s := range states {
			statusCell := string(s.Status)
			switch s.Status {
			case fleet.StatusRunning:
				statusCell = p.Paint(statusCell, termcolor.Green)
			case fleet.StatusFailed:
				statusCell = p.Paint(statusCell, termcolor.Red)
			case fleet.StatusStopped:
				statusCell = p.Paint(statusCell, termcolor.Yellow)
			case fleet.StatusComplete:
				statusCell = p.Paint(statusCell, termcolor.Cyan)
			}
			tbl.AddRow(s.Name, s.Agent, statusCell, fmt.Sprintf("%d", s.PID), s.Branch)
		}
		tbl.Render(cmd.OutOrStdout())
		return nil
	},
}

var fleetStopCmd = &cobra.Command{
	Use:   "stop <task>",
	Short: "Stop a running fleet task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		r := newRunner()

		mgr, err := fleet.NewManager(r, cfg, nil)
		if err != nil {
			return err
		}

		if err := mgr.Stop(args[0]); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Stopped task %q\n", args[0])
		return nil
	},
}

var fleetLogsCmd = &cobra.Command{
	Use:   "logs <task>",
	Short: "Show logs for a fleet task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.FromContext(cmd.Context())
		r := newRunner()

		mgr, err := fleet.NewManager(r, cfg, nil)
		if err != nil {
			return err
		}

		logPath, err := mgr.Logs(args[0])
		if err != nil {
			return err
		}

		data, err := os.ReadFile(logPath)
		if err != nil {
			return fmt.Errorf("read log: %w", err)
		}

		fmt.Fprint(cmd.OutOrStdout(), string(data))
		return nil
	},
}

// makeCommandFactory creates a CommandFactory that resolves agent adapters.
func makeCommandFactory() fleet.CommandFactory {
	return func(agentName, dir string, spec fleet.TaskSpec) *exec.Cmd {
		a := agent.Resolve(agentName)
		return a.Command(dir, spec)
	}
}

// completeFleetTasks returns task names from fleet state for completion.
func completeFleetTasks(cmd *cobra.Command, _ string) ([]string, cobra.ShellCompDirective) { //nolint:unparam // ShellCompDirective return is a Cobra convention
	cfg := config.FromContext(cmd.Context())
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	r := newRunner()
	mgr, err := fleet.NewManager(r, cfg, nil)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	states, err := mgr.Status()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, s := range states {
		names = append(names, s.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// init registers completion functions for fleet subcommands.
func init() { //nolint:gochecknoinits // Cobra convention
	fleetStopCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		all, dir := completeFleetTasks(cmd, toComplete)
		var filtered []string
		for _, n := range all {
			if strings.HasPrefix(n, toComplete) {
				filtered = append(filtered, n)
			}
		}
		return filtered, dir
	}
	fleetLogsCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		all, dir := completeFleetTasks(cmd, toComplete)
		var filtered []string
		for _, n := range all {
			if strings.HasPrefix(n, toComplete) {
				filtered = append(filtered, n)
			}
		}
		return filtered, dir
	}
}
