package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lugassawan/rimba/internal/errhint"
)

const notInRepoHint = "run from inside a git repository, or run: git init"

const (
	refsHeadsPrefix = "refs/heads/"
	cmdRevParse     = "rev-parse"
	flagVerify      = "--verify"
	cmdCommitTree   = "commit-tree"
	cmdCherry       = "cherry"
	cmdConfig       = "config"
	treeSuffix      = "^{tree}"
	cherryMerged    = "- "
)

// RepoRoot returns the absolute path to the repository root.
func RepoRoot(ctx context.Context, r Runner) (string, error) {
	out, err := r.Run(ctx, cmdRevParse, "--show-toplevel")
	if err != nil {
		return "", errhint.WithFix(fmt.Errorf("not a git repository: %w", err), notInRepoHint)
	}
	return out, nil
}

// RepoName returns the name of the repository (basename of the root directory).
func RepoName(ctx context.Context, r Runner) (string, error) {
	root, err := RepoRoot(ctx, r)
	if err != nil {
		return "", err
	}
	return filepath.Base(root), nil
}

// MainRepoRoot returns the absolute path to the main repository root.
// Unlike RepoRoot, this always returns the main repo root even when called
// from within a worktree. Uses --git-common-dir whose parent is the main root.
func MainRepoRoot(ctx context.Context, r Runner) (string, error) {
	commonDir, err := resolveCommonDir(ctx, r)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Dir(commonDir)), nil
}

// HooksDir returns the absolute path to the repository's hooks directory.
// Respects core.hooksPath if configured, otherwise falls back to
// <git-common-dir>/hooks for worktree compatibility.
func HooksDir(ctx context.Context, r Runner) (string, error) {
	// Check if core.hooksPath is configured (overrides default)
	hooksPath, err := r.Run(ctx, cmdConfig, "core.hooksPath")
	if err == nil && hooksPath != "" {
		if filepath.IsAbs(hooksPath) {
			return hooksPath, nil
		}
		// Relative path: resolve against main repo root
		root, err := MainRepoRoot(ctx, r)
		if err != nil {
			return "", err
		}
		return filepath.Join(root, hooksPath), nil
	}

	// Default: <git-common-dir>/hooks
	commonDir, err := resolveCommonDir(ctx, r)
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "hooks"), nil
}

// DefaultBranch detects the default branch (main or master).
func DefaultBranch(ctx context.Context, r Runner) (string, error) {
	// Try symbolic-ref for origin/HEAD first
	out, err := r.Run(ctx, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// refs/remotes/origin/main → main
		parts := strings.Split(out, "/")
		return parts[len(parts)-1], nil
	}

	// Fall back: check if main exists
	if _, err := r.Run(ctx, cmdRevParse, flagVerify, refsHeadsPrefix+"main"); err == nil {
		return "main", nil
	}

	// Fall back: check if master exists
	if _, err := r.Run(ctx, cmdRevParse, flagVerify, refsHeadsPrefix+"master"); err == nil {
		return "master", nil
	}

	return "", errhint.WithFix(
		errors.New("could not detect default branch (no main or master found)"),
		"set the default branch: git branch -M main",
	)
}

// resolveCommonDir returns the absolute path to the git common directory.
// --git-common-dir may return a relative path; this resolves it against the repo root.
func resolveCommonDir(ctx context.Context, r Runner) (string, error) {
	commonDir, err := r.Run(ctx, cmdRevParse, "--git-common-dir")
	if err != nil {
		return "", errhint.WithFix(fmt.Errorf("not a git repository: %w", err), notInRepoHint)
	}

	if !filepath.IsAbs(commonDir) {
		root, err := RepoRoot(ctx, r)
		if err != nil {
			return "", err
		}
		commonDir = filepath.Join(root, commonDir)
	}

	return commonDir, nil
}
