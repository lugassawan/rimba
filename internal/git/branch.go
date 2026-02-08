package git

import (
	"strings"
)

// BranchExists checks whether a local branch exists.
func BranchExists(r Runner, branch string) bool {
	_, err := r.Run(cmdRevParse, flagVerify, refsHeadsPrefix+branch)
	return err == nil
}

// DeleteBranch deletes a local branch. If force is true, uses -D instead of -d.
func DeleteBranch(r Runner, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.Run("branch", flag, branch)
	return err
}

// RenameBranch renames a local branch from oldBranch to newBranch.
func RenameBranch(r Runner, oldBranch, newBranch string) error {
	_, err := r.Run("branch", "-m", oldBranch, newBranch)
	return err
}

// IsDirty returns true if the working tree at the given directory has uncommitted changes.
func IsDirty(r Runner, dir string) (bool, error) {
	out, err := r.RunInDir(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// AheadBehind returns the ahead/behind counts of the current branch vs its upstream.
// Returns (0, 0, nil) if there's no upstream configured.
func AheadBehind(r Runner, dir string) (ahead, behind int, _ error) {
	out, err := r.RunInDir(dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		// No upstream or other error — treat as 0/0
		return 0, 0, nil //nolint:nilerr // intentional: missing upstream is not an error
	}

	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, nil
	}

	var a, b int
	parseCount(parts[0], &b)
	parseCount(parts[1], &a)
	return a, b, nil
}

// MergedBranches returns branches that have been merged into the given branch.
// Runs `git branch --merged <branch>` and parses the output.
func MergedBranches(r Runner, branch string) ([]string, error) {
	out, err := r.Run("branch", "--merged", branch)
	if err != nil {
		return nil, err
	}

	var branches []string
	for line := range strings.SplitSeq(out, "\n") {
		// Lines are "  branch-name", "* current-branch", or "+ worktree-branch"
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "+ ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func parseCount(s string, v *int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	*v = n
	return n
}
