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
func AheadBehind(r Runner, dir string) (ahead, behind int, err error) {
	out, err := r.RunInDir(dir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		// No upstream or other error — treat as 0/0
		return 0, 0, nil
	}

	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, nil
	}

	var a, b int
	_, _ = parseCount(parts[0], &b)
	_, _ = parseCount(parts[1], &a)
	return a, b, nil
}

func parseCount(s string, v *int) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	*v = n
	return n, nil
}
