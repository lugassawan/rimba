package git

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	refsHeadsPrefix = "refs/heads/"
	cmdRevParse     = "rev-parse"
	flagVerify      = "--verify"
	cmdCommitTree   = "commit-tree"
	cmdCherry       = "cherry"
	treeSuffix      = "^{tree}"
	cherryMerged    = "- "
)

// RepoRoot returns the absolute path to the repository root.
func RepoRoot(r Runner) (string, error) {
	out, err := r.Run(cmdRevParse, "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return out, nil
}

// RepoName returns the name of the repository (basename of the root directory).
func RepoName(r Runner) (string, error) {
	root, err := RepoRoot(r)
	if err != nil {
		return "", err
	}
	return filepath.Base(root), nil
}

// MainRepoRoot returns the absolute path to the main repository root.
// Unlike RepoRoot, this always returns the main repo root even when called
// from within a worktree. Uses --git-common-dir whose parent is the main root.
func MainRepoRoot(r Runner) (string, error) {
	commonDir, err := resolveCommonDir(r)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Dir(commonDir)), nil
}

// HooksDir returns the absolute path to the repository's hooks directory.
// Uses git rev-parse --git-common-dir for worktree compatibility.
func HooksDir(r Runner) (string, error) {
	commonDir, err := resolveCommonDir(r)
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "hooks"), nil
}

// resolveCommonDir returns the absolute path to the git common directory.
// --git-common-dir may return a relative path; this resolves it against the repo root.
func resolveCommonDir(r Runner) (string, error) {
	commonDir, err := r.Run(cmdRevParse, "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}

	if !filepath.IsAbs(commonDir) {
		root, err := RepoRoot(r)
		if err != nil {
			return "", err
		}
		commonDir = filepath.Join(root, commonDir)
	}

	return commonDir, nil
}

// DefaultBranch detects the default branch (main or master).
func DefaultBranch(r Runner) (string, error) {
	// Try symbolic-ref for origin/HEAD first
	out, err := r.Run("symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// refs/remotes/origin/main → main
		parts := strings.Split(out, "/")
		return parts[len(parts)-1], nil
	}

	// Fall back: check if main exists
	if _, err := r.Run(cmdRevParse, flagVerify, refsHeadsPrefix+"main"); err == nil {
		return "main", nil
	}

	// Fall back: check if master exists
	if _, err := r.Run(cmdRevParse, flagVerify, refsHeadsPrefix+"master"); err == nil {
		return "master", nil
	}

	return "", errors.New("could not detect default branch (no main or master found)")
}
