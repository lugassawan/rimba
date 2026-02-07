package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	refsHeadsPrefix = "refs/heads/"
	cmdRevParse     = "rev-parse"
	flagVerify      = "--verify"
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

	return "", fmt.Errorf("could not detect default branch (no main or master found)")
}
