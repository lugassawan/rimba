package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/agentfile"
	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/errhint"
	"github.com/lugassawan/rimba/internal/fileutil"
	"github.com/lugassawan/rimba/internal/git"
	"github.com/spf13/cobra"
)

const (
	flagPersonal  = "personal"
	flagGlobal    = "global"
	flagAgents    = "agents"
	flagLocal     = "local"
	flagUninstall = "uninstall"
)

type installTier string

const (
	tierUser         installTier = "user"
	tierProject      installTier = "project"
	tierProjectLocal installTier = "project-local"
)

const (
	localConfigHint = "check directory permissions for .rimba/ in the repo root"
	gitignoreHint   = "check write permissions for .gitignore in the repo root"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize rimba in the current repository",
	Example: `  rimba init
  rimba init --personal
  rimba init --agents
  rimba init -g`,
	Long: `Detects the repository root and sets up the .rimba/ config directory with
settings.toml (team-shared) and settings.local.toml (personal overrides).

If a legacy .rimba.toml file exists, it is migrated into the new directory layout.
Use --agents to also install AI agent instruction files at project level (committed).
Use --agents --local to install them gitignored (personal overrides).
Use -g to install agent files at user level (~/) — works outside a git repository.
Use -g --uninstall to remove user-level files, --agents --uninstall for project-team files,
or --agents --local --uninstall for project-local files.
Use --personal to gitignore the entire .rimba/ directory instead of just the local
config file. In personal mode, settings.local.toml is not created since the whole
directory is already personal.

When --agents or -g is used, rimba also registers itself as an MCP server (server name:
rimba, command: rimba mcp) in client config files (.mcp.json, .cursor/mcp.json,
~/.claude.json, ~/.codex/config.toml, ~/.gemini/settings.json,
~/.codeium/windsurf/mcp_config.json, ~/.roo/mcp.json). --agents --local does not
register MCP — it only updates agent files. Registration is idempotent.`,
	Annotations: map[string]string{"skipConfig": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		m := initModeFromFlags(cmd)
		if err := m.validate(); err != nil {
			return err
		}

		// Global (-g) branch: no repo required.
		if m.global {
			return runInitGlobal(cmd, m.uninstall)
		}

		r := newRunner(cmd.Context())

		ctx := cmd.Context()
		repoRoot, err := git.RepoRoot(ctx, r)
		if err != nil {
			return err
		}

		personal, _ := cmd.Flags().GetBool(flagPersonal)

		dirPath := filepath.Join(repoRoot, config.DirName)
		legacyPath := filepath.Join(repoRoot, config.FileName)
		globEntry := config.DirName + "/" + config.LocalGlob
		dirEntry := config.DirName + "/"

		gitignoreEntry := globEntry
		if personal {
			gitignoreEntry = dirEntry
		}

		switch {
		case dirExists(dirPath):
			// .rimba/ directory already exists
			fmt.Fprintf(cmd.OutOrStdout(), "Config %s already exists, skipping config creation\n", dirPath)
			if err := reconcileExistingIgnore(cmd, repoRoot, personal); err != nil {
				return err
			}

		case fileExists(legacyPath):
			if err := runInitMigrate(cmd, repoRoot, dirPath, legacyPath, gitignoreEntry, personal); err != nil {
				return err
			}

		default:
			if err := runInitFresh(ctx, cmd, r, repoRoot, dirPath, gitignoreEntry, personal); err != nil {
				return err
			}
		}

		// Project-level agent files
		if m.agents {
			return runInitAgents(cmd, repoRoot, m.local, m.uninstall)
		}

		return nil
	},
}

func runInitFresh(ctx context.Context, cmd *cobra.Command, r git.Runner, repoRoot, dirPath, gitignoreEntry string, personal bool) error {
	repoName, err := git.RepoName(ctx, r)
	if err != nil {
		return err
	}

	defaultBranch, err := git.DefaultBranch(ctx, r)
	if err != nil {
		return err
	}

	copyFiles, autoDetected := detectCopyFiles(ctx, cmd, r, repoRoot)
	cfg := &config.Config{
		CopyFiles: copyFiles,
	}

	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return errhint.WithFix(
			fmt.Errorf("failed to create config directory: %w", err),
			localConfigHint,
		)
	}

	if err := config.Save(filepath.Join(dirPath, config.TeamFile), cfg); err != nil {
		return errhint.WithFix(fmt.Errorf("failed to save team config: %w", err), localConfigHint)
	}

	if !personal {
		if err := os.WriteFile(filepath.Join(dirPath, config.LocalFile), nil, 0600); err != nil {
			return errhint.WithFix(fmt.Errorf("failed to create local config: %w", err), localConfigHint)
		}
	}

	wtDir := filepath.Join(repoRoot, config.DefaultWorktreeDir(repoName))
	if err := os.MkdirAll(wtDir, 0750); err != nil {
		return errhint.WithFix(
			fmt.Errorf("failed to create worktree directory: %w", err),
			"check directory permissions for the worktree dir, or set worktree.dir in .rimba/settings.toml",
		)
	}

	added, err := ensureLocalIgnore(repoRoot, gitignoreEntry, personal)
	if err != nil {
		return errhint.WithFix(fmt.Errorf("failed to update .gitignore: %w", err), gitignoreHint)
	}

	if !personal {
		if _, err := fileutil.EnsureGitignore(repoRoot, config.DirName+"/"+metricsFileName); err != nil {
			return errhint.WithFix(fmt.Errorf("failed to update .gitignore: %w", err), gitignoreHint)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialized rimba in %s\n", repoRoot)
	fmt.Fprintf(cmd.OutOrStdout(), "  Config:       %s\n", filepath.Join(dirPath, config.TeamFile))
	fmt.Fprintf(cmd.OutOrStdout(), "  Worktree dir: %s\n", wtDir)
	fmt.Fprintf(cmd.OutOrStdout(), "  Source:       %s\n", defaultBranch)
	copyFilesNote := ""
	if !autoDetected {
		copyFilesNote = " (default)"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Copy files:   %s%s\n", strings.Join(cfg.CopyFiles, ", "), copyFilesNote)
	if added {
		fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:    %s added to .gitignore\n", gitignoreEntry)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:    %s (already in .gitignore)\n", gitignoreEntry)
	}
	return nil
}

// detectCopyFiles falls back to config.DefaultCopyFiles() when the scan is
// empty or fails; scan failures are non-fatal so init never fails.
func detectCopyFiles(ctx context.Context, cmd *cobra.Command, r git.Runner, repoRoot string) (files []string, autoDetected bool) {
	candFiles, candDirs := config.CandidateCopyFiles()
	pathspecs := append(append([]string{}, candFiles...), candDirs...)

	ignored, err := git.ListIgnoredUntracked(ctx, r, repoRoot, pathspecs)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: copy_files detection failed: %v — using defaults\n", err)
		return config.DefaultCopyFiles(), false
	}

	if detected := config.DetectCopyFiles(ignored); len(detected) > 0 {
		return detected, true
	}
	return config.DefaultCopyFiles(), false
}

func runInitMigrate(cmd *cobra.Command, repoRoot, dirPath, legacyPath, gitignoreEntry string, personal bool) error {
	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return errhint.WithFix(
			fmt.Errorf("failed to create config directory: %w", err),
			localConfigHint,
		)
	}

	if err := os.Rename(legacyPath, filepath.Join(dirPath, config.TeamFile)); err != nil {
		return errhint.WithFix(
			fmt.Errorf("failed to move legacy config: %w", err),
			"check write permissions for the repo root and move .rimba.toml to .rimba/settings.toml manually",
		)
	}

	if !personal {
		if err := os.WriteFile(filepath.Join(dirPath, config.LocalFile), nil, 0600); err != nil {
			return errhint.WithFix(fmt.Errorf("failed to create local config: %w", err), localConfigHint)
		}
	}

	_, _ = fileutil.RemoveGitignoreEntry(repoRoot, config.FileName)

	if _, err := ensureLocalIgnore(repoRoot, gitignoreEntry, personal); err != nil {
		return errhint.WithFix(fmt.Errorf("failed to update .gitignore: %w", err), gitignoreHint)
	}

	if !personal {
		if _, err := fileutil.EnsureGitignore(repoRoot, config.DirName+"/"+metricsFileName); err != nil {
			return errhint.WithFix(fmt.Errorf("failed to update .gitignore: %w", err), gitignoreHint)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Migrated rimba config in %s\n", repoRoot)
	fmt.Fprintf(cmd.OutOrStdout(), "  Moved:     %s → %s\n", config.FileName, filepath.Join(config.DirName, config.TeamFile))
	if !personal {
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:   %s\n", filepath.Join(config.DirName, config.LocalFile))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore: updated (%s → %s)\n", config.FileName, gitignoreEntry)
	return nil
}

func runInitAgents(cmd *cobra.Command, repoRoot string, local, uninstall bool) error {
	if uninstall {
		if local {
			return runInstall(cmd, repoRoot, tierProjectLocal, "Removed", agentfile.UninstallLocal, nil)
		}
		return runInstall(cmd, repoRoot, tierProject, "Removed", agentfile.UninstallProject, agentfile.UnregisterMCPProject)
	}
	if local {
		return runInstall(cmd, repoRoot, tierProjectLocal, "Installed", agentfile.InstallLocal, nil)
	}
	return runInstall(cmd, repoRoot, tierProject, "Installed", agentfile.InstallProject, agentfile.RegisterMCPProject)
}

func runInitGlobal(cmd *cobra.Command, uninstall bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errhint.WithFix(
			fmt.Errorf("resolve home directory: %w", err),
			"set HOME to your user home directory (e.g. /home/<username> on Linux, /Users/<username> on macOS)",
		)
	}
	if uninstall {
		return runInstall(cmd, home, tierUser, "Removed", agentfile.UninstallGlobal, agentfile.UnregisterMCPGlobal)
	}
	return runInstall(cmd, home, tierUser, "Installed", agentfile.InstallGlobal, agentfile.RegisterMCPGlobal)
}

// runInstall runs installFn(dir) and optionally mcpFn(dir), then prints results.
// mcpFn may be nil (local tier skips MCP registration).
func runInstall(
	cmd *cobra.Command,
	dir string,
	tier installTier,
	verb string,
	installFn func(string) ([]agentfile.Result, error),
	mcpFn func(string) ([]agentfile.Result, error),
) error {
	files, err := installFn(dir)
	if err != nil {
		return errhint.WithFix(
			fmt.Errorf("agent files: %w", err),
			"check write permissions for the install dir",
		)
	}
	var mcps []agentfile.Result
	if mcpFn != nil {
		mcps, err = mcpFn(dir)
		if err != nil {
			return errhint.WithFix(
				fmt.Errorf("mcp servers: %w", err),
				"check write permissions for MCP client configs (.mcp.json, .cursor/mcp.json, ~/.claude.json)",
			)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s rimba (%s):\n", verb, tier)
	if len(files) > 0 {
		printSection(cmd, "Agent files", tier, files)
	}
	if len(mcps) > 0 {
		printSection(cmd, "MCP server", tier, mcps)
	}
	if n := countCorrupt(files) + countCorrupt(mcps); n > 0 {
		return fmt.Errorf("%d file(s) have a corrupt rimba block — resolve manually", n)
	}
	return nil
}

// countCorrupt reports how many results have a corrupt rimba block.
func countCorrupt(results []agentfile.Result) int {
	n := 0
	for _, r := range results {
		if r.Corrupt {
			n++
		}
	}
	return n
}

func printSection(cmd *cobra.Command, title string, tier installTier, results []agentfile.Result) {
	fmt.Fprintf(cmd.OutOrStdout(), "  %s:\n", title)
	for _, r := range results {
		p := r.RelPath
		if tier == tierUser {
			p = "~/" + p
		}
		action := r.Action
		if r.Corrupt {
			action = "corrupt — resolve manually"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "    %s (%s)\n", p, action)
	}
}

// ensureLocalIgnore updates .gitignore for local config files.
// Personal mode ignores the whole .rimba/ dir; non-personal mode writes the
// *.local.toml glob. gitignoreEntry is only used in the personal branch.
func ensureLocalIgnore(repoRoot, gitignoreEntry string, personal bool) (bool, error) {
	if personal {
		return fileutil.EnsureGitignore(repoRoot, gitignoreEntry)
	}
	return fileutil.EnsureLocalGlobIgnored(repoRoot)
}

// reconcileExistingIgnore migrates per-file .gitignore entries to the glob on
// re-init. Switching from non-personal to personal mode is not handled here.
func reconcileExistingIgnore(cmd *cobra.Command, repoRoot string, personal bool) error {
	if personal {
		return nil
	}
	added, err := fileutil.EnsureLocalGlobIgnored(repoRoot)
	if err != nil {
		return errhint.WithFix(fmt.Errorf("failed to update .gitignore: %w", err), gitignoreHint)
	}
	if added {
		fmt.Fprintf(cmd.OutOrStdout(), "  Gitignore:    %s added to .gitignore\n", config.DirName+"/"+config.LocalGlob)
	}

	if _, err := fileutil.EnsureGitignore(repoRoot, config.DirName+"/"+metricsFileName); err != nil {
		return errhint.WithFix(fmt.Errorf("failed to update .gitignore: %w", err), gitignoreHint)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool(flagPersonal, false, "gitignore the .rimba/ directory (for solo developers)")
	initCmd.Flags().BoolP(flagGlobal, "g", false, "install rimba agent files at user level (~/)")
	initCmd.Flags().Bool(flagAgents, false, "install rimba agent files in this project (committed to repo)")
	initCmd.Flags().Bool(flagLocal, false, "with --agents, install as project-local (gitignored)")
	initCmd.Flags().Bool(flagUninstall, false, "remove agent files: use with -g, --agents, or --agents --local")
	initCmd.MarkFlagsMutuallyExclusive(flagGlobal, flagAgents)
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
