package cmd

import (
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	mcppkg "github.com/lugassawan/rimba/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(mcpCmd)
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI tool integration",
	Long: `Starts a Model Context Protocol (MCP) server over stdio.

AI tools like Claude Code and Cursor can connect to this server to discover
and invoke rimba commands with structured parameters and typed responses.

To configure in Claude Code, add to .claude/settings.json:
  {"mcpServers": {"rimba": {"command": "rimba", "args": ["mcp"]}}}`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner()

		repoRoot, err := git.MainRepoRoot(r)
		if err != nil {
			return err
		}

		// Config is optional — some tools work without it.
		cfg, _ := config.Resolve(repoRoot)
		if cfg != nil {
			repoName := filepath.Base(repoRoot)
			var defaultBranch string
			if cfg.DefaultSource == "" {
				defaultBranch, _ = git.DefaultBranch(r)
			}
			cfg.FillDefaults(repoName, defaultBranch)
		}

		hctx := &mcppkg.HandlerContext{
			Runner:   r,
			Config:   cfg,
			RepoRoot: repoRoot,
			Version:  version,
		}

		s := mcppkg.NewServer(hctx)
		return server.ServeStdio(s)
	},
}
