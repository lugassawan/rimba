package cmd

import (
	"path/filepath"

	"github.com/lugassawan/rimba/internal/config"
	"github.com/lugassawan/rimba/internal/git"
)

// resolveMainBranch tries to get the main branch from config, falling back to DefaultBranch.
func resolveMainBranch(r git.Runner) (string, error) {
	repoRoot, err := git.RepoRoot(r)
	if err != nil {
		return "", err
	}

	cfg, err := config.Load(filepath.Join(repoRoot, configFileName))
	if err == nil && cfg.DefaultSource != "" {
		return cfg.DefaultSource, nil
	}

	// No config — use git detection
	return git.DefaultBranch(r)
}
