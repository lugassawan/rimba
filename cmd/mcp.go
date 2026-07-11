package cmd

import (
	"errors"
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
	mcppkg "github.com/lugassawan/rimba/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:     "mcp",
	Short:   "Start MCP server for AI tool integration",
	Example: "  rimba mcp",
	Long: `Starts a Model Context Protocol (MCP) server over stdio.

Any MCP-compatible client can connect to this server to discover and invoke
rimba commands with structured parameters and typed responses.`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r := newRunner(cmd.Context())

		repoRoot, err := git.MainRepoRoot(cmd.Context(), r)
		if err != nil {
			return err
		}

		// Config is optional — absence is tolerated, real load errors are not.
		cfg, err := config.Resolve(repoRoot)
		if err != nil && !errors.Is(err, config.ErrConfigAbsent) {
			return err
		}
		if cfg != nil {
			repoName := filepath.Base(repoRoot)
			defaultBranch, _ := git.DefaultBranch(cmd.Context(), r)
			cfg.FillDefaults(repoName, defaultBranch)
		}

		hctx := &mcppkg.HandlerContext{
			Runner:   r,
			GH:       newGHRunner(cmd.Context()),
			Config:   cfg,
			RepoRoot: repoRoot,
			Version:  version,
		}

		s := mcppkg.NewServer(hctx)
		return server.ServeStdio(s)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
