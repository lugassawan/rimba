package git

import "strings"

// DiffNameOnly returns the files changed between base and branch.
// Uses three-dot diff (base...branch) to compare against the merge-base.
func DiffNameOnly(r Runner, base, branch string) ([]string, error) {
	out, err := r.Run("diff", "--name-only", base+"..."+branch)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var files []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// MergeTree performs an in-memory merge check using git merge-tree (git 2.38+).
// Returns true if the merge would have conflicts, without modifying the working tree.
func MergeTree(r Runner, base, branch1, branch2 string) (hasConflict bool, _ error) {
	out, err := r.Run("merge-tree", "--write-tree", "--merge-base="+base, branch1, branch2)
	if err != nil {
		// Non-zero exit from merge-tree means conflicts were detected.
		// The output will contain conflict markers or informational messages.
		if strings.Contains(out, "CONFLICT") {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
